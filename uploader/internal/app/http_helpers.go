package app

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/cors"
)

func CORS(conf *CORSConfig) func(next http.Handler) http.Handler {
	c := cors.New(cors.Options{
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Origin", "Accept", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders: []string{"Content-Disposition"},
		AllowOriginFunc: func(origin string) bool {
			u, err := url.Parse(origin)
			if err != nil {
				return false
			}

			for _, h := range conf.AllowedHosts {
				if u.Host == h {
					return true
				}
			}

			for _, s := range conf.AllowedHostSuffixes {
				if strings.HasSuffix(u.Host, s) {
					return true
				}
			}

			return false
		},
		AllowCredentials: true,
	})

	return func(next http.Handler) http.Handler {
		return c.Handler(next)
	}
}

var (
	xForwardedForY = "X-Forwarded-For-Y"
	xForwardedFor  = "X-Forwarded-For"
)

// Origin extracts original IP address of a client.
//
// When connecting to a web server through an HTTP proxy or a load balancer
// the address from X-Forwarded-For-Y or standard X-Forwarded-For header is used.
func Origin(r *http.Request) string {
	if h := r.Header.Get(xForwardedForY); h != "" {
		return h
	} else if h := r.Header.Get(xForwardedFor); h != "" {
		return h
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}
