package gotor

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/cgi"
	"net/http/fcgi"
	"sync"

	"github.com/caddyserver/certmagic"
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

var changeDefaultACMEMtx sync.Mutex

func newTLSConfig(domains []string, email string) (*tls.Config, error) {
	changeDefaultACMEMtx.Lock()

	certmagic.DefaultACME.Agreed = true
	certmagic.DefaultACME.Email = email

	cfg := certmagic.NewDefault()

	changeDefaultACMEMtx.Unlock()

	err := cfg.ManageSync(context.Background(), domains)
	if err != nil {
		return nil, err
	}

	tlsConfig := cfg.TLSConfig()
	tlsConfig.NextProtos = append([]string{"h2", "http/1.1"}, tlsConfig.NextProtos...)

	return tlsConfig, nil
}

func HTTPS(addr string, domains []string, email string, hf http.HandlerFunc) error {
	tlsConfig, err := newTLSConfig(domains, email)
	if err != nil {
		return err
	}
	lnr, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	return http.Serve(lnr, hfShell(hf))
}

func ServeHTTPS(lnr net.Listener, domains []string, email string, hf http.HandlerFunc) error {
	tlsConfig, err := newTLSConfig(domains, email)
	if err != nil {
		return err
	}
	return http.Serve(tls.NewListener(lnr, tlsConfig), hfShell(hf))
}

func HTTPSWithHTTP(addr string, domains []string, email string, hf http.HandlerFunc) error {
	host, port, err := net.SplitHostPort(addr)
	if err == nil && port == "443" {
		go HTTP(net.JoinHostPort(host, "80"), RedirectionSite("https://"+domains[0], http.StatusTemporaryRedirect))
	}
	return HTTPS(addr, domains, email, hf)
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
