package gotor

import (
	"net/http"
)

func Run(addr string, hf http.HandlerFunc) {
	http.ListenAndServe(addr, h)
}

func RunTLS(addr string, certFile string, keyFile string, hf http.HandlerFunc) {
	http.ListenAndServeTLS(addr, certFile, keyFile, h)
}

var NotFound http.HandlerFunc = http.NotFound
