package gotor

import (
	"net/http"
	"io"
	"strings"
	"compress/gzip"
)

type nopCloser struct {
	io.Writer
}

func (*nopCloser) Close() error {
	return nil
}

type httpResponseWriter struct {
	http.ResponseWriter
	req *http.Request
	wc io.WriteCloser
	wh bool
}

func newHTTPRW(srcResp http.ResponseWriter, req *http.Request) *httpResponseWriter {
	return &httpResponseWriter{srcResp, req, &nopCloser{srcResp}, false}
}

func (rw *httpResponseWriter) WriteHeader(status int) {
	if rw.Header().Get("Content-Encoding") == "gzip" {
		if strings.Index(rw.req.Header.Get("Accept-Encoding"), "gzip") != -1 {
			z := gzip.NewWriter(rw.ResponseWriter)
			rw.wc = z
		} else {
			rw.Header().Del("Content-Encoding")
		}
	}
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *httpResponseWriter) Write(data []byte) (int, error) {
	if !rw.wh {
		rw.WriteHeader(http.StatusOK)
		rw.wh = true
	}
	return rw.wc.Write(data)
}