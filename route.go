package hs

import (
	"net/http"
)

type Route map[string]Handler

func PathRouter(m Route) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		h, ok := m[r.URL.Path]
		if ok {
			h(w, r)
			return
		}
		for i := len(r.URL.Path) - 1; i > 0; i-- {
			if r.URL.Path[i] == '/' {
				h, ok = m[r.URL.Path[:i]]
				if ok {
					q := r.URL.Query()
					q.Set("@rest", r.URL.Path[i:])
					r.URL.RawQuery = q.Encode()
					h(w, r)
					return
				}
			}
		}
		http.NotFound(w, r)
	}
}

func matchRoute(w http.ResponseWriter, r *http.Request, m Route, key string) {
	h, ok := m[key]
	if ok {
		h(w, r)
		return
	}
	h, ok = m[""]
	if ok {
		h(w, r)
		return
	}
	http.NotFound(w, r)
}

func MethodRouter(m Route) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		matchRoute(w, r, m, r.Method)
	}
}

func HostRouter(m Route) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		matchRoute(w, r, m, r.Host)
	}
}
