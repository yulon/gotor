package gotor

import (
	"net/http"
)

func Run(addr string, hf http.HandlerFunc) {
	http.ListenAndServe(addr, hf)
}

func RunTLS(addr string, certFile string, keyFile string, hf http.HandlerFunc) {
	http.ListenAndServeTLS(addr, certFile, keyFile, hf)
}

var NotFound http.HandlerFunc = http.NotFound
