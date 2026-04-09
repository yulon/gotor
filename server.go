package gotor

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/cgi"
	"net/http/fcgi"

	"github.com/caddyserver/certmagic"
)

var NotFound http.HandlerFunc = http.NotFound

func SmartHandler(src http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rw := newSmartRespWriter(w, r)
		src.ServeHTTP(rw, r)
		rw.Close()
	}
}

func HTTP(addr string, h http.Handler) error {
	return http.ListenAndServe(addr, SmartHandler(h))
}

func ServeHTTP(lnr net.Listener, h http.Handler) error {
	return http.Serve(lnr, SmartHandler(h))
}

func NewTLSConfig(email string, domains []string, dnsProvider certmagic.DNSProvider) (*tls.Config, error) {
	cfg := certmagic.NewDefault()

	isr := certmagic.NewACMEIssuer(cfg, certmagic.DefaultACME)
	isr.Agreed = true
	isr.Email = email
	isr.Profile = "shortlived"
	if dnsProvider != nil {
		isr.DNS01Solver = &certmagic.DNS01Solver{
			DNSManager: certmagic.DNSManager{
				DNSProvider: dnsProvider,
			},
		}
	}

	cfg.Issuers = append(cfg.Issuers, isr)

	err := cfg.ManageSync(context.Background(), domains)
	if err != nil {
		return nil, err
	}

	tlsConfig := cfg.TLSConfig()
	tlsConfig.NextProtos = append([]string{"h2", "http/1.1"}, tlsConfig.NextProtos...)

	return tlsConfig, nil
}

func listenTLS(addr string, email string, domains []string, dnsProvider certmagic.DNSProvider) (net.Listener, error) {
	tlsConfig, err := NewTLSConfig(email, domains, dnsProvider)
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

func HTTPS(addr string, email string, dnsProvider certmagic.DNSProvider, domainHandlers HostRouter) error {
	lnr, err := listenTLS(addr, email, getDomains(domainHandlers), dnsProvider)
	if err != nil {
		return err
	}
	return http.Serve(lnr, SmartHandler(domainHandlers))
}

func ServeHTTPS(lnr net.Listener, email string, dnsProvider certmagic.DNSProvider, domainHandlers HostRouter) error {
	tlsConfig, err := NewTLSConfig(email, getDomains(domainHandlers), dnsProvider)
	if err != nil {
		return err
	}
	return http.Serve(tls.NewListener(lnr, tlsConfig), SmartHandler(domainHandlers))
}

func HTTPSWithHTTP(addr string, email string, dnsProvider certmagic.DNSProvider, domainHandlers HostRouter) error {
	domains := getDomains(domainHandlers)
	host, port, err := net.SplitHostPort(addr)
	if err == nil && port == "443" {
		go http.ListenAndServe(net.JoinHostPort(host, "80"), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, _, err := net.SplitHostPort(r.Host)
			if err != nil {
				host = r.Host
			}
			Redirect(w, "https://"+host+r.RequestURI, http.StatusPermanentRedirect)
		}))
	}
	lnr, err := listenTLS(addr, email, domains, dnsProvider)
	if err != nil {
		return err
	}
	return http.Serve(lnr, SmartHandler(domainHandlers))
}

func CGI(h http.HandlerFunc) error {
	return cgi.Serve(SmartHandler(h))
}

func FastCGI(addr string, h http.HandlerFunc) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return fcgi.Serve(ln, SmartHandler(h))
}

func ServeFastCGI(lnr net.Listener, h http.HandlerFunc) error {
	return fcgi.Serve(lnr, SmartHandler(h))
}
