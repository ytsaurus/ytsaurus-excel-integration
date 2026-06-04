package app

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/sync/errgroup"

	"go.ytsaurus.tech/library/go/core/log"
	"go.ytsaurus.tech/library/go/httputil/middleware/httpmetrics"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"
	"go.ytsaurus.tech/yt/microservices/excel/pkg/events"
)

const (
	httpServerGracefulStopTimeout = 30 * time.Second
	ssoCookieForwardedName        = "access_token"
)

// App is a god object that manages service lifetime.
type App struct {
	conf *Config
	l    log.Structured

	metrics *MetricsRegistry
}

// NewApp creates new app.
func NewApp(c *Config, l log.Structured) *App {
	return &App{
		conf:    c,
		l:       l,
		metrics: NewMetricsRegistry(),
	}
}

// Run performs initialization and starts all components.
//
// Can be canceled via context.
func (a *App) Run(ctx context.Context) error {
	a.l.Info("starting app")
	defer func() {
		a.l.Info("app stopped")
	}()

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		a.runHTTPServer(gctx, a.newDebugHTTPServer())
		return gctx.Err()
	})

	r := chi.NewMux()
	r.Use(httpmetrics.New(a.metrics.WithPrefix("http")))
	r.Use(timeout(a.conf.HTTPHandlerTimeout))
	r.Use(requestLog(a.l, int64(a.conf.MaxExcelFileSize)))
	r.Use(CORS(a.conf.CORS))

	var ew events.Writer = events.NoOpWriter{}
	if ec := a.conf.Events; ec != nil && ec.Enabled {
		if ec.LogPattern == "" {
			return fmt.Errorf("events.log_pattern is required when events.enabled is true")
		}
		rotationTime := ec.RotationTime
		if rotationTime == 0 {
			rotationTime = 15 * time.Minute
		}
		maxAge := ec.MaxAge
		if maxAge == 0 {
			maxAge = 7 * 24 * time.Hour
		}
		fw, err := events.NewFileWriter(ec.LogPattern, ec.LinkName, rotationTime, maxAge, events.SourceExcelUploader, a.l)
		if err != nil {
			return fmt.Errorf("creating event writer: %w", err)
		}
		ew = fw
	}

	for _, c := range a.conf.Clusters {
		l := log.With(a.l.Logger(), log.String("cluster", c.Proxy)).Structured()
		yc, err := ythttp.NewClient(&yt.Config{
			Proxy:   c.Proxy,
			UseTLS:  c.UseTLS,
			Logger:  l,
			TraceFn: events.YTTraceFn,
		})
		if err != nil {
			return err
		}

		api := NewAPI(c, yc, a.l, ew)
		apiRouter := r.
			With(ForwardCookie(a.conf.AuthCookieName)).
			With(ForwardCookieRenamed(a.conf.SSOCookieName, ssoCookieForwardedName)).
			With(ForwardUserTicket)

		clusterMetrics := a.metrics.WithTags(map[string]string{"yt-cluster": c.Proxy})
		api.RegisterMetrics(clusterMetrics)
		apiRouter.Mount(path.Join("/", a.conf.APIPathPrefix, c.APIEndpointName, "api"), api.Routes())
		api.SetReady()
	}

	server := &http.Server{
		Addr:    a.conf.HTTPAddr,
		Handler: r,
	}

	g.Go(func() error {
		a.runHTTPServer(gctx, server)
		return gctx.Err()
	})

	return g.Wait()
}

func (a *App) newDebugHTTPServer() *http.Server {
	debugRouter := chi.NewMux()
	debugRouter.Handle("/debug/*", http.DefaultServeMux)
	a.metrics.HandleMetrics(debugRouter)
	return &http.Server{
		Addr:    a.conf.DebugHTTPAddr,
		Handler: debugRouter,
	}
}

// runHTTPServer runs http server and gracefully stop it when the context is closed.
func (a *App) runHTTPServer(ctx context.Context, s *http.Server) {
	a.l.Info("starting http server", log.String("addr", s.Addr))

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		err := s.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	<-ctx.Done()

	a.l.Info("waiting for http server to stop",
		log.String("addr", a.conf.HTTPAddr), log.Duration("timeout", httpServerGracefulStopTimeout))

	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpServerGracefulStopTimeout)
	defer cancel()
	if err := s.Shutdown(shutdownCtx); err != nil {
		if err == context.DeadlineExceeded {
			a.l.Warn("http server shutdown deadline exceeded",
				log.String("addr", a.conf.HTTPAddr))
		} else {
			panic(err)
		}
	}

	wg.Wait()

	a.l.Info("http server stopped", log.String("addr", s.Addr))
}
