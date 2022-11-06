package gotor

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

const halfUint64 uint64 = uint64(18446744073709551615) / uint64(2)

type slowReader struct {
	r      io.Reader
	spdLmt uint64
	rn     uint64
	rnt    time.Time
}

func newSlowReader(r io.Reader, speedLimit uint64) io.Reader {
	if speedLimit <= 0 {
		return r
	}
	return &slowReader{r: r, spdLmt: speedLimit}
}

func (lr slowReader) Read(p []byte) (int, error) {
	if lr.rn == 0 {
		lr.rnt = time.Now()
	}
	n, err := lr.Read(p)
	if n <= 0 {
		return n, err
	}
	lr.rn += uint64(n)
	s := time.Now().Sub(lr.rnt).Seconds()
	if s >= 1 || lr.rn > halfUint64 {
		exceedN := lr.rn - uint64(s*float64(lr.spdLmt))
		if exceedN > 0 {
			time.Sleep(time.Duration(exceedN*uint64(1000)/lr.spdLmt) * time.Millisecond)
		}
		lr.rn = 0
	}
	return n, err
}

func pxyAuthCredDecBasic(cont string) []string {
	b := make([]byte, len(cont)*2)
	n, err := base64.RawStdEncoding.Decode(b, []byte(strings.TrimSpace(cont)))
	if err != nil || n == 0 {
		return nil
	}
	parts := strings.SplitN(string(b[:n]), ":", 2)
	if len(parts) != 2 {
		return nil
	}
	parts[0] = strings.TrimSpace(parts[0])
	parts[1] = strings.TrimSpace(parts[1])
	return parts
}

var otherPxyAuthCredDecs = map[string]func(string) []string{}

type ProxyValve struct {
	UploadLimit   uint64
	DownloadLimit uint64
}

func Proxy(tp *http.Transport, valve *ProxyValve, auth func(user, passwd string) *ProxyValve) http.HandlerFunc {
	if tp == nil {
		tp = &http.Transport{
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var userValve *ProxyValve = nil

		if auth != nil {
			pa := r.Header.Get("Proxy-Authorization")
			if len(pa) == 0 {
				w.Header().Set("Proxy-Authenticate", "Basic")
				w.WriteHeader(http.StatusProxyAuthRequired)
				return
			}
			r.Header.Del("Proxy-Authorization")

			paParts := strings.SplitN(pa, " ", 2)
			if len(paParts) != 2 {
				w.Header().Set("Proxy-Authenticate", "Basic")
				w.WriteHeader(http.StatusProxyAuthRequired)
				return
			}
			pat := strings.TrimSpace(paParts[0])
			pac := strings.TrimSpace(paParts[1])

			var up []string
			if pat == "Basic" {
				up = pxyAuthCredDecBasic(pac)
			} else {
				pacDec, ok := otherPxyAuthCredDecs[pat]
				if !ok {
					w.Header().Set("Proxy-Authenticate", "Basic")
					w.WriteHeader(http.StatusProxyAuthRequired)
					return
				}
				up = pacDec(pac)
			}
			if len(up) != 2 {
				w.Header().Set("Proxy-Authenticate", "Basic")
				w.WriteHeader(http.StatusProxyAuthRequired)
				return
			}

			userValve = auth(up[0], up[1])
			if userValve == nil {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			if valve != nil {
				if userValve.UploadLimit < 0 && valve.UploadLimit >= 0 {
					userValve.UploadLimit = valve.UploadLimit
				}
				if userValve.DownloadLimit < 0 && valve.DownloadLimit >= 0 {
					userValve.DownloadLimit = valve.DownloadLimit
				}
			}
		} else if valve != nil {
			userValve = valve
		}

		if r.Method == "CONNECT" {
			tarCon, err := tp.DialContext(context.Background(), "tcp", r.RequestURI)
			if err != nil {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			defer tarCon.Close()

			pxyCon, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			defer pxyCon.Close()

			pxyCon.Write([]byte(r.Proto + " 200 Connection Established\r\n\r\n"))

			if userValve != nil {
				go io.Copy(tarCon, newSlowReader(pxyCon, userValve.UploadLimit))
				io.Copy(pxyCon, newSlowReader(tarCon, userValve.DownloadLimit))
				return
			}
			go io.Copy(tarCon, pxyCon)
			io.Copy(pxyCon, tarCon)
			return
		}

		r.Header.Set("Connection", r.Header.Get("Proxy-Connection"))
		r.Header.Del("Proxy-Connection")

		r.RequestURI = ""
		resp, err := tp.RoundTrip(r) // TODO: use userValve.UploadLimit
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for k := range resp.Header {
			w.Header().Set(k, resp.Header.Get(k))
		}
		w.WriteHeader(resp.StatusCode)

		if userValve != nil {
			io.Copy(w, newSlowReader(resp.Body, userValve.DownloadLimit))
			return
		}
		io.Copy(w, resp.Body)
	}
}

type proxyRedirectResponseWriter struct {
	http.ResponseWriter
	tarURL *url.URL
	reqURL *url.URL
}

func (prrw *proxyRedirectResponseWriter) WriteHeader(status int) {
	rustr := prrw.Header().Get("Location")
	if rustr != "" {
		ru, err := url.Parse(rustr)
		if err != nil && ru.Scheme == prrw.tarURL.Scheme && ru.Host == prrw.tarURL.Host && len(ru.RawPath) >= len(prrw.tarURL.Path) && ru.RawPath[:len(prrw.tarURL.Path)] == prrw.tarURL.Path {
			ru.Host = prrw.reqURL.Host
			ru.RawPath = ru.RawPath[len(prrw.tarURL.Path):]
			prrw.Header().Set("Location", ru.String())
		}
	}
	prrw.ResponseWriter.WriteHeader(status)
}

func ReverseProxy(target string, fixHost, fixRedirect bool) http.HandlerFunc {
	tu, err := url.Parse(target)
	if err != nil {
		return nil
	}
	shrp := httputil.NewSingleHostReverseProxy(tu)
	if fixHost {
		if fixRedirect {
			return func(w http.ResponseWriter, r *http.Request) {
				r.Host = tu.Host
				shrp.ServeHTTP(&proxyRedirectResponseWriter{w, tu, r.URL}, r)
			}
		}
		return func(w http.ResponseWriter, r *http.Request) {
			r.Host = tu.Host
			shrp.ServeHTTP(w, r)
		}
	}
	if fixRedirect {
		return func(w http.ResponseWriter, r *http.Request) {
			shrp.ServeHTTP(&proxyRedirectResponseWriter{w, tu, r.URL}, r)
		}
	}
	return shrp.ServeHTTP
}
