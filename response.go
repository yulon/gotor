package gotor

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
)

type nopCloser struct {
	io.Writer
}

func (*nopCloser) Close() error {
	return nil
}

type smartRespWriter struct {
	http.ResponseWriter
	req       *http.Request
	enc       io.WriteCloser
	isWritten bool
	status    int
	wantBr    bool
	wantGz    bool
}

func newSmartRespWriter(srcResp http.ResponseWriter, req *http.Request) *smartRespWriter {
	return &smartRespWriter{srcResp, req, nil, false, -1, false, false}
}

func (srw *smartRespWriter) WriteHeader(status int) {
	srw.status = status
}

func (srw *smartRespWriter) Write(data []byte) (int, error) {
	if srw.enc != nil {
		return srw.enc.Write(data)
	}
	if !srw.isWritten {
		srw.isWritten = true
		if srw.status < 0 {
			srw.status = http.StatusOK
		}
		if len(srw.Header().Get("Content-Length")) == 0 {
			ce := strings.ToLower(srw.Header().Get("Content-Encoding"))
			switch ce {
			case "br":
				if strings.Contains(strings.ToLower(srw.req.Header.Get("Accept-Encoding")), "br") {
					srw.wantBr = true
					break
				}
				srw.Header().Set("Content-Encoding", "gzip")
				fallthrough
			case "gzip":
				if strings.Contains(strings.ToLower(srw.req.Header.Get("Accept-Encoding")), "gzip") {
					srw.wantGz = true
				} else {
					srw.Header().Del("Content-Encoding")
					srw.ResponseWriter.WriteHeader(srw.status)
				}
			}
		} else {
			srw.ResponseWriter.WriteHeader(srw.status)
		}
	}
	if len(data) == 0 {
		return 0, nil
	}
	if srw.wantBr {
		srw.wantBr = false
		srw.ResponseWriter.WriteHeader(srw.status)
		srw.enc = brotli.NewWriter(srw.ResponseWriter)
		return srw.enc.Write(data)
	}
	if srw.wantGz {
		srw.wantGz = false
		srw.ResponseWriter.WriteHeader(srw.status)
		srw.enc = gzip.NewWriter(srw.ResponseWriter)
		return srw.enc.Write(data)
	}
	return srw.ResponseWriter.Write(data)
}

func (srw *smartRespWriter) Close() error {
	if !srw.isWritten {
		srw.ResponseWriter.WriteHeader(srw.status)
		return nil
	}
	if srw.enc != nil {
		err := srw.enc.Close()
		srw.enc = nil
		return err
	}
	if srw.wantBr {
		srw.wantBr = false
		srw.Header().Del("Content-Encoding")
		srw.Header().Set("Content-Length", "0")
		srw.ResponseWriter.WriteHeader(srw.status)
		return nil
	}
	if srw.wantGz {
		srw.wantGz = false
		srw.Header().Del("Content-Encoding")
		srw.Header().Set("Content-Length", "0")
		srw.ResponseWriter.WriteHeader(srw.status)
		return nil
	}
	return nil
}
