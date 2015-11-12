package gotor

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

type ReverseProxy struct{
	Host bool
	Redirect bool
}

type proxyRedirectResponseWriter struct{
	http.ResponseWriter
	tarURL *url.URL
	reqURL *url.URL
}

func (prrw *proxyRedirectResponseWriter) WriteHeader(status int) {
	rustr := prrw.Header().Get("Location")
	if rustr != "" {
		ru, err := url.Parse(rustr)
		if err != nil && ru.Scheme == prrw.tarURL.Scheme && ru.Host == prrw.tarURL.Host && len(ru.RawPath) >= len(prrw.tarURL.Path) && ru.RawPath[:len(prrw.tarURL.Path)] == prrw.tarURL.Path {
			ru.Host = prrw.reqURL.Host
			ru.RawPath = ru.RawPath[len(prrw.tarURL.Path):]
			prrw.Header().Set("Location", ru.String())
		}
	}
	prrw.ResponseWriter.WriteHeader(status)
}

func (rp *ReverseProxy) Pass(target string) Handler {
	tu, err := url.Parse(target)
	if err != nil {
		return nil
	}
	shrp := httputil.NewSingleHostReverseProxy(tu)
	switch {
		case rp.Host && rp.Redirect:
			return func(w http.ResponseWriter, r *http.Request) {
				r.Host = tu.Host
				shrp.ServeHTTP(&proxyRedirectResponseWriter{w, tu, r.URL}, r)
			}
		case rp.Host:
			return func(w http.ResponseWriter, r *http.Request) {
				r.Host = tu.Host
				shrp.ServeHTTP(w, r)
			}
		case rp.Redirect:
			return func(w http.ResponseWriter, r *http.Request) {
				shrp.ServeHTTP(&proxyRedirectResponseWriter{w, tu, r.URL}, r)
			}
	}
	return shrp.ServeHTTP
}
