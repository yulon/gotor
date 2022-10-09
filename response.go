package gotor

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type nopCloser struct {
	io.Writer
}

func (*nopCloser) Close() error {
	return nil
}

type responseWriter struct {
	http.ResponseWriter
	req *http.Request
	wc  io.WriteCloser
	wh  bool
}

func newResponseWriter(srcResp http.ResponseWriter, req *http.Request) *responseWriter {
	return &responseWriter{srcResp, req, &nopCloser{srcResp}, false}
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.wh = true
	if rw.Header().Get("Content-Encoding") == "gzip" && len(rw.Header().Get("Content-Length")) == 0 {
		if strings.Contains(rw.req.Header.Get("Accept-Encoding"), "gzip") {
			z := gzip.NewWriter(rw.ResponseWriter)
			rw.wc = z
		} else {
			rw.Header().Del("Content-Encoding")
		}
	}
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	if !rw.wh {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.wc.Write(data)
}
