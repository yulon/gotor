package gotor

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

func ProxyPass(target string) Handler {
	t, err := url.Parse(target)
	if err != nil {
		return nil
	}
	rp := httputil.NewSingleHostReverseProxy(t)
	return func(w http.ResponseWriter, r *http.Request) {
		r.Host = t.Host
		rp.ServeHTTP(w, r)
	}
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

func ProxyRedirect(target string) Handler {
	t, err := url.Parse(target)
	if err != nil {
		return nil
	}
	rp := httputil.NewSingleHostReverseProxy(t)
	return func(w http.ResponseWriter, r *http.Request) {
		r.Host = t.Host
		rp.ServeHTTP(&proxyRedirectResponseWriter{w, t, r.URL}, r)
	}
}
