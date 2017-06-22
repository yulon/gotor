package gotor

import (
	"net"
	"net/http"
	"net/http/cgi"
	"net/http/fcgi"
)

var NotFound http.HandlerFunc = http.NotFound

func hfShell(src http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rw := newHTTPRW(w, r)
		src(rw, r)
		rw.wc.Close()
	}
}

func HTTP(addr string, hf http.HandlerFunc) error {
	return http.ListenAndServe(addr, hfShell(hf))
}

func HTTPS(addr string, certFile string, keyFile string, hf http.HandlerFunc) error {
	return http.ListenAndServeTLS(addr, certFile, keyFile, hfShell(hf))
}

func CGI(hf http.HandlerFunc) error {
	return cgi.Serve(hfShell(hf))
}

func FastCGI(addr string, hf http.HandlerFunc) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return fcgi.Serve(ln, hfShell(hf))
}
