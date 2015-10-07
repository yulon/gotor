package hs

import (
	"net/http"
)

func Redirect(w http.ResponseWriter, r *http.Request, url string, code int) {
	w.Header().Set("Location", url)
	w.WriteHeader(code)
}

func RedirectHandler(url string, code int) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		Redirect(w, r, url, code)
	}
}

func RedirectHostHandler(host string, code int) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Host = host
		Redirect(w, r, r.URL.String(), code)
	}
}

func RedirectHostRootHandler(host string, code int) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Host = host
		r.URL.Path = r.URL.Query().Get("@rest")
		Redirect(w, r, r.URL.String(), code)
	}
}
