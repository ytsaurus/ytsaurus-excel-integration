package app

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/sync/errgroup"

	"go.ytsaurus.tech/library/go/core/log"
	"go.ytsaurus.tech/library/go/httputil/middleware/httpmetrics"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"
)

const httpServerGracefulStopTimeout = 30 * time.Second

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
	r.Use(CORS())

	for _, c := range a.conf.Clusters {
		l := log.With(a.l.Logger(), log.String("cluster", c.Proxy)).Structured()
		yc, err := ythttp.NewClient(&yt.Config{
			Proxy:  c.Proxy,
			Logger: l,
		})
		if err != nil {
			return err
		}

		api := NewAPI(c, yc, a.l)
		apiRouter := r.With(ForwardCookie(a.conf.AuthCookieName)).With(ForwardUserTicket)
		clusterMetrics := a.metrics.WithTags(map[string]string{"yt-cluster": c.Proxy})
		api.RegisterMetrics(clusterMetrics)
		apiRouter.Mount("/"+c.APIEndpointName+"/api", api.Routes())
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
