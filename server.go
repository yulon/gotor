package gotor

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/cgi"
	"net/http/fcgi"
	"strconv"
	"strings"
	"sync"

	"github.com/caddyserver/certmagic"
)

var NotFound http.HandlerFunc = http.NotFound

func cvtHandler(src http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rw := newResponseWriter(w, r)
		src.ServeHTTP(rw, r)
		rw.wc.Close()
	}
}

func HTTP(addr string, h http.Handler) error {
	return http.ListenAndServe(addr, cvtHandler(h))
}

func ServeHTTP(lnr net.Listener, h http.Handler) error {
	return http.Serve(lnr, cvtHandler(h))
}

var changeDefaultACMEMtx sync.Mutex

func newTLSConfig(email string, domains []string) (*tls.Config, error) {
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

func listenTLS(addr string, email string, domains []string) (net.Listener, error) {
	tlsConfig, err := newTLSConfig(email, domains)
	if err != nil {
		return nil, err
	}
	return tls.Listen("tcp", addr, tlsConfig)
}

func getDomains(domainHandlers HostRouter) []string {
	var domains []string
	for domain := range domainHandlers {
		domains = append(domains, domain)
	}
	return domains
}

func HTTPS(addr string, email string, domainHandlers HostRouter) error {
	lnr, err := listenTLS(addr, email, getDomains(domainHandlers))
	if err != nil {
		return err
	}
	return http.Serve(lnr, cvtHandler(domainHandlers))
}

func ServeHTTPS(lnr net.Listener, email string, domainHandlers HostRouter) error {
	tlsConfig, err := newTLSConfig(email, getDomains(domainHandlers))
	if err != nil {
		return err
	}
	return http.Serve(tls.NewListener(lnr, tlsConfig), cvtHandler(domainHandlers))
}

var http2HTTPSCode = []byte(`<html><head><script type="text/javascript">location.protocol='https:'</script></head><body></body></html>`)

var http2HTTPSCodeLenStr = strconv.Itoa(len(http2HTTPSCode))

func getGzipHTTP2HTTPSCode() []byte {
	b := bytes.NewBuffer([]byte{})
	z := gzip.NewWriter(b)
	z.Write(http2HTTPSCode)
	z.Close()
	return b.Bytes()
}

var gzipHTTP2HTTPSCode = getGzipHTTP2HTTPSCode()

var gzipHTTP2HTTPSCodeLenStr = strconv.Itoa(len(gzipHTTP2HTTPSCode))

func HTTPSWithHTTP(addr string, email string, domainHandlers HostRouter) error {
	domains := getDomains(domainHandlers)
	host, port, err := net.SplitHostPort(addr)
	if err == nil && port == "443" {
		var h80 http.HandlerFunc
		if len(domains) == 1 {
			h80 = RedirectionSite("https://"+domains[0], http.StatusTemporaryRedirect)
		} else {
			h80 = func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=UTF-8")
				w.Header().Set("Cache-Control", "no-cache")
				if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
					w.Header().Set("Content-Encoding", "gzip")
					w.Header().Set("Content-Length", gzipHTTP2HTTPSCodeLenStr)
					w.WriteHeader(http.StatusOK)
					w.Write(gzipHTTP2HTTPSCode)
					return
				}
				w.Header().Set("Content-Length", http2HTTPSCodeLenStr)
				w.WriteHeader(http.StatusOK)
				w.Write(http2HTTPSCode)
			}
		}
		go http.ListenAndServe(net.JoinHostPort(host, "80"), h80)
	}
	lnr, err := listenTLS(addr, email, domains)
	if err != nil {
		return err
	}
	return http.Serve(lnr, cvtHandler(domainHandlers))
}

func CGI(h http.HandlerFunc) error {
	return cgi.Serve(cvtHandler(h))
}

func FastCGI(addr string, h http.HandlerFunc) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return fcgi.Serve(ln, cvtHandler(h))
}

func ServeFastCGI(lnr net.Listener, h http.HandlerFunc) error {
	return fcgi.Serve(lnr, cvtHandler(h))
}
