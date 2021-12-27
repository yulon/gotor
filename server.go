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

func ServeHTTP(lnr net.Listener, hf http.HandlerFunc) error {
	return http.Serve(lnr, hfShell(hf))
}

func HTTPS(addr string, certFile string, keyFile string, hf http.HandlerFunc) error {
	return http.ListenAndServeTLS(addr, certFile, keyFile, hfShell(hf))
}

func ServeHTTPS(lnr net.Listener, certFile string, keyFile string, hf http.HandlerFunc) error {
	return http.ServeTLS(lnr, hfShell(hf), certFile, keyFile)
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

func ServeFastCGI(lnr net.Listener, hf http.HandlerFunc) error {
	return fcgi.Serve(lnr, hfShell(hf))
}
