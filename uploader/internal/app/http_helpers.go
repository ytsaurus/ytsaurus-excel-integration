package app

import (
	"net"
	"net/http"
)

func CORS() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// CORS middleware

			next.ServeHTTP(w, r)
		})
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
