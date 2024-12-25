package gotor

import (
	"net"
	"net/http"
	"strings"
)

type PathRouter map[string]http.Handler

func (m PathRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h, ok := m[r.URL.Path]
	if ok {
		h.ServeHTTP(w, r)
		return
	}
	for i := len(r.URL.Path) - 1; i >= 0; i-- {
		if r.URL.Path[i] == '/' {
			h, ok = m[r.URL.Path[:i+1]+"*"]
			if ok {
				q := r.URL.Query()
				q.Set("*", r.URL.Path[i+1:])
				r.URL.RawQuery = q.Encode()
				h.ServeHTTP(w, r)
				return
			}
		}
	}
	NotFound(w, r)
}

func matchRoute(w http.ResponseWriter, r *http.Request, m map[string]http.Handler, key string) {
	h, ok := m[key]
	if ok {
		h.ServeHTTP(w, r)
		return
	}
	h, ok = m["*"]
	if ok {
		h.ServeHTTP(w, r)
		return
	}
	NotFound(w, r)
}

type MethodRouter map[string]http.Handler

func (m MethodRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	matchRoute(w, r, m, r.Method)
}

type HostRouter map[string]http.Handler

func (m HostRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		return
	}
	matchRoute(w, r, m, host)
}

type UserAgentRouter map[string]http.Handler

func (m UserAgentRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dh, ok := m["*"]
	if !ok {
		dh = NotFound
	}
	ua := r.UserAgent()
	for k, h := range m {
		if strings.Index(ua, k) != -1 {
			h.ServeHTTP(w, r)
			return
		}
	}
	dh.ServeHTTP(w, r)
}

func DeviceRouter(pc http.Handler, mobile http.Handler) http.Handler {
	return UserAgentRouter{
		"*":       pc,
		"Mobile":  mobile,
		"Android": mobile,
	}
}
