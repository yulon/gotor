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
					if r.URL.RawQuery == "" {
						r.URL.RawQuery += "&"
					}
					r.URL.RawQuery += ":*=" + r.URL.Path[i:]
					h(w, r)
					return
				}
			}
		}
		http.NotFound(w, r)
	}
}

func MethodRouter(m Route) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		h, ok := m[r.Method]
		if ok {
			h(w, r)
			return
		}
		h, ok = m["*"]
		if ok {
			h(w, r)
			return
		}
		http.NotFound(w, r)
	}
}

func HostRouter(m Route) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		h, ok := m[r.Host]
		if !ok {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}
}
