package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang/protobuf/proto"
	"go.uber.org/atomic"
	"go.ytsaurus.tech/library/go/core/log"
	"go.ytsaurus.tech/library/go/core/log/ctxlog"
	"go.ytsaurus.tech/yt/go/guid"
	"go.ytsaurus.tech/yt/go/proto/core/rpc"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yterrors"
	"golang.org/x/xerrors"
)

const (
	// YT balancer's header.
	xReqIDHTTPHeader = "X-Req-Id"

	xYTError           = "X-YT-Error"
	xYTResponseCode    = "X-YT-Response-Code"
	xYTResponseMessage = "X-YT-Response-Message"

	xCSRFHTTPHeader = "X-Csrf-Token"
)

// requestLog logs
//   - http method, path, query, body
//   - generated guid
//   - balancer's request id (X-Req-Id header)
//   - request execution time, status and number of bytes written
//   - original client IP
func requestLog(l log.Structured) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := guid.New()
			requestIDField := log.String("request_id", requestID.String())

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			resp := &bytes.Buffer{}
			ww.Tee(resp)

			const bodySizeLimit = 1024 * 1024 * 50
			body, err := io.ReadAll(io.LimitReader(r.Body, bodySizeLimit))
			if err != nil {
				l.Error("error reading request body", log.Error(err))
			}
			r.Body = io.NopCloser(bytes.NewBuffer(body))
			_ = r.ParseForm()
			r.Body = io.NopCloser(bytes.NewBuffer(body))

			l.Debug("HTTP request started",
				requestIDField,
				log.String("method", r.Method),
				log.String("path", r.URL.Path),
				log.String("query", r.Form.Encode()),
				log.String("origin", Origin(r)),
				log.String("l7_req_id", r.Header.Get(xReqIDHTTPHeader)))

			t0 := time.Now()
			defer func() {
				l.Debug("HTTP request finished",
					requestIDField,
					log.Int("status", ww.Status()),
					log.String(xYTError, ww.Header().Get(xYTError)),
					log.String(xYTResponseCode, ww.Header().Get(xYTResponseCode)),
					log.String(xYTResponseMessage, ww.Header().Get(xYTResponseMessage)),
					log.Int("bytes", ww.BytesWritten()),
					log.Duration("duration", time.Since(t0)))
			}()

			ctx := ctxlog.WithFields(r.Context(), requestIDField)
			ctx = withRequestID(ctx, requestID)
			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
}

func timeout(timeout time.Duration) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// requestIDKey is a key used to access request id in request's ctx.
var requestIDKey struct{}

// withRequestID copies given context and adds (*requestIDKey, reqID) to values.
func withRequestID(ctx context.Context, reqID guid.GUID) context.Context {
	return context.WithValue(ctx, &requestIDKey, reqID)
}

// contextRequestID retrieves request id from context.
func contextRequestID(ctx context.Context) (reqID guid.GUID) {
	val := ctx.Value(&requestIDKey)
	if val != nil {
		reqID = val.(guid.GUID)
	}
	return
}

func waitReady(ready *atomic.Bool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !ready.Load() {
				replyError(w, r, xerrors.New("not ready, try later"), http.StatusInternalServerError)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// cookieCredentials is an implementation of yt.Credentials that
// adds a cookie and csrf token to the request.
type cookieCredentials struct {
	cookie    *http.Cookie
	csrfToken string
}

func (c cookieCredentials) Set(r *http.Request) {
	r.AddCookie(c.cookie)
	r.Header.Set("X-Csrf-Token", c.csrfToken)
}

func (c cookieCredentials) SetExtension(req *rpc.TRequestHeader) {
	_ = proto.SetExtension(
		req,
		rpc.E_TCredentialsExt_CredentialsExt,
		&rpc.TCredentialsExt{SessionId: &c.cookie.Value},
	)
}

// sessionIDCookie is a name of a cookie which is forwarded from frontend to yt proxy.
//
// There will be no need in this action when tvm support is added to proxy (https://st.yandex-team.ru/YT-4570). // todo
const sessionIDCookie = "Session_id"

// ForwardCookie creates a middleware that extracts specific cookie and adds it to request context.
func ForwardCookie(name string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(name)
			if err == nil {
				credentials := cookieCredentials{
					cookie:    cookie,
					csrfToken: r.Header.Get(xCSRFHTTPHeader),
				}
				ctx := yt.WithCredentials(r.Context(), credentials)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// XYaUserTicket is http header that should be used for user ticket transfer.
const XYaUserTicket = "X-Ya-User-Ticket"

// ForwardUserTicket is a middleware that extracts X-Ya-User-Ticket header and adds it to request context.
func ForwardUserTicket(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ticket := r.Header.Get(XYaUserTicket)
		if ticket != "" {
			ctx := yt.WithCredentials(r.Context(), &yt.UserTicketCredentials{Ticket: ticket})
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

var host, _ = os.Hostname()

func replyError(w http.ResponseWriter, r *http.Request, err error, status int) {
	ytErr := yterrors.FromError(err).(*yterrors.Error)
	ytErr.AddAttr("host", host)
	ytErr.AddAttr("request_id", contextRequestID(r.Context()))

	js, _ := json.Marshal(ytErr)
	w.Header().Add(xYTError, string(js))
	w.Header().Add(xYTResponseCode, strconv.Itoa(int(ytErr.Code)))
	w.Header().Add(xYTResponseMessage, ytErr.Message)

	w.WriteHeader(status)

	js, _ = json.MarshalIndent(ytErr, "", "  ")
	_, _ = w.Write(js)
}