package hs

import (
	"net/http"
)

type Handler func(http.ResponseWriter, *http.Request)

type slh struct{
	h Handler
}

func (s *slh) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.h(w, r)
}

func ToStdHandler(h Handler) http.Handler {
	return &slh{h}
}

func New(addr string, h Handler) {
	http.ListenAndServe(addr, ToStdHandler(h))
}

func NewTLS(addr string, certFile string, keyFile string, h Handler) {
	http.ListenAndServeTLS(addr, certFile, keyFile, ToStdHandler(h))
}

var NotFound Handler = http.NotFound
