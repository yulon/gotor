package gotor

import (
	"net"
	"net/http"
	"net/http/cgi"
	"net/http/fcgi"
)

var NotFound http.HandlerFunc = http.NotFound

func HTTP(addr string, hf http.HandlerFunc) error {
	return http.ListenAndServe(addr, hf)
}

func HTTPS(addr string, certFile string, keyFile string, hf http.HandlerFunc) error {
	return http.ListenAndServeTLS(addr, certFile, keyFile, hf)
}

func CGI(hf http.HandlerFunc) error {
	return cgi.Serve(hf)
}

func FastCGI(addr string, hf http.HandlerFunc) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return fcgi.Serve(ln, hf)
}
