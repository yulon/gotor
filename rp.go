package gotor

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

func ProxyPass(w http.ResponseWriter, r *http.Request, target string) {
	t, _ := url.Parse(target)
	rp := httputil.NewSingleHostReverseProxy(t)
	r.Host = t.Host
	rp.ServeHTTP(w, r)
}

func ProxyPassHandler(target string) Handler {
	t, _ := url.Parse(target)
	rp := httputil.NewSingleHostReverseProxy(t)
	return func(w http.ResponseWriter, r *http.Request) {
		r.Host = t.Host
		rp.ServeHTTP(w, r)
	}
}
