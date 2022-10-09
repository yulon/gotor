package gotor

import (
	"net/http"
)

func Redirect(w http.ResponseWriter, url string, code int) {
	w.Header().Set("Location", url)
	w.WriteHeader(code)
}

func Redirection(url string, code int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		Redirect(w, url, code)
	}
}

func RedirectionSite(site string, code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		Redirect(w, site+r.RequestURI, code)
	}
}
