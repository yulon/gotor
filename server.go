package hs

import (
	"net/http"
)

type Handler func(http.ResponseWriter, *http.Request)

type slh struct{
	h Handler
	s string
}

func (s *slh) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.URL.Scheme = s.s
	s.h(w, r)
}

func StdLibHandler(h Handler) http.Handler {
	return &slh{h, ""}
}

func StdLibHandlerHaveScheme(h Handler, scheme string) http.Handler {
	return &slh{h, scheme}
}

func New(addr string, h Handler) {
	http.ListenAndServe(addr, StdLibHandlerHaveScheme(h, "http"))
}

func NewTLS(addr string, certFile string, keyFile string, h Handler) {
	http.ListenAndServeTLS(addr, certFile, keyFile, StdLibHandlerHaveScheme(h, "https"))
}
