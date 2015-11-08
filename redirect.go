package gotor

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

func RedirectSiteHandler(site string, code int) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		Redirect(w, r, site + r.RequestURI, code)
	}
}
