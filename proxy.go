package gotor

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/yulon/go-netil"
	"github.com/yulon/gocks5"
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

func hostToAddr(proto, host string) (addr string) {
	addr = host
	colonPos := strings.LastIndexByte(addr, ':')
	if colonPos < 0 || colonPos < strings.LastIndexByte(addr, ']') {
		switch proto {
		case "http":
			addr += ":80"
		case "https":
			addr += ":443"
		}
	}
	return
}

func URLToAddr(u *url.URL) string {
	return hostToAddr(strings.ToLower(u.Scheme), u.Host)
}

func basicAuthEnc(u *url.Userinfo) string {
	auth := u.Username()
	pw, hasPw := u.Password()
	if hasPw {
		auth += ":" + pw
	}
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func basicAuthDec(s string) *url.Userinfo {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(b) == 0 {
		return nil
	}
	auth := strings.TrimSpace(string(b))
	if len(auth) == 0 {
		return nil
	}
	parts := strings.SplitN(string(b), ":", 2)
	if len(parts) != 2 {
		return url.User(strings.TrimSpace(parts[0]))
	}
	return url.UserPassword(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
}

var otherAuthDecs = map[string]func(string) *url.Userinfo{}

func DialPass(proxy *url.URL, addr net.Addr, dial func(addr net.Addr) (net.Conn, error)) (net.Conn, error) {
	if proxy == nil {
		return netil.DialOrDirect(addr, dial)
	}
	proto := strings.ToLower(proxy.Scheme)
	switch proto {
	case "socks5":
		u := proxy.User.Username()
		p, _ := proxy.User.Password()

		if dial == nil {
			con, _, _, err := gocks5.DialTCPPass(proxy.Host, u, p, addr)
			return con, err
		}

		pxyRawCon, err := dial(netil.DomainAddr(proxy.Host))
		if err != nil {
			return nil, err
		}
		pxyCon, _, err := gocks5.PassRawConn(pxyRawCon, u, p)
		if err != nil {
			return nil, err
		}
		con, _, err := pxyCon.DialTCP(addr)
		return con, err

	case "http":
		fallthrough
	case "https":
		saddr := hostToAddr(proto, proxy.Host)
		auth := basicAuthEnc(proxy.User)
		if len(auth) > 0 {
			auth = "Basic " + auth
		}

		var pxyCon net.Conn
		var err error
		if dial != nil {
			pxyCon, err = dial(netil.DomainAddr(saddr))
		} else {
			pxyCon, err = net.Dial("tcp", saddr)
		}
		if err != nil {
			return nil, err
		}

		req := &http.Request{
			Method: "CONNECT",
			URL:    &url.URL{Opaque: addr.String()},
		}
		if len(auth) > 0 {
			req.Header.Set("Proxy-Authorization", auth)
		}
		err = req.Write(pxyCon)
		if err != nil {
			return nil, err
		}

		pxyR := bufio.NewReader(pxyCon)
		resp, err := http.ReadResponse(pxyR, req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != 200 {
			errStr := http.StatusText(resp.StatusCode)
			if resp.StatusCode == http.StatusProxyAuthRequired {
				errStr += " " + resp.Header.Get("Proxy-Authenticate")
			}
			return nil, errors.New(errStr)
		}

		return pxyCon, nil
	}
	return nil, errors.New("unsupported " + proto)
}

type ProxyConnConfig struct {
	Transport *http.Transport
	/*UpLimiter   *gocks5.SpeedLimiter
	DownLimiter *gocks5.SpeedLimiter*/
}

type Proxy struct {
	OnRequest func(r *http.Request, user *url.Userinfo) (bool, *ProxyConnConfig)
	Transport *http.Transport
}

func getUser(r *http.Request, field string) *url.Userinfo {
	pa := r.Header.Get(field)
	if len(pa) == 0 {
		return nil
	}

	paParts := strings.SplitN(pa, " ", 2)
	if len(paParts) != 2 {
		return nil
	}
	pat := strings.TrimSpace(paParts[0])
	pac := strings.TrimSpace(paParts[1])

	if pat == "Basic" {
		return basicAuthDec(pac)
	}
	pacDec, ok := otherAuthDecs[pat]
	if !ok {
		return nil
	}
	return pacDec(pac)
}

func GetProxyUser(r *http.Request) *url.Userinfo {
	return getUser(r, "Proxy-Authorization")
}

var proxyDefaultTransport = &http.Transport{
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

func (pxy *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tp := pxy.Transport
	if tp == nil {
		tp = proxyDefaultTransport
	}

	u := GetProxyUser(r)
	var pcc *ProxyConnConfig
	if pxy.OnRequest != nil {
		var ok bool
		ok, pcc = pxy.OnRequest(r, u)
		if !ok {
			if u == nil {
				w.Header().Set("Proxy-Authenticate", "Basic")
				w.WriteHeader(http.StatusProxyAuthRequired)
				return
			}
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if pcc.Transport != nil {
			tp = pcc.Transport
		}
	}

	if r.Method == "CONNECT" {
		svrAddr := URLToAddr(r.URL)
		var svrCon net.Conn
		var err error
		if tp.Proxy != nil {
			var proxy *url.URL
			proxy, err = tp.Proxy(r)
			if err != nil {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			if proxy != nil {
				var dial func(addr net.Addr) (net.Conn, error)
				if tp.DialContext != nil {
					dial = func(addr net.Addr) (net.Conn, error) {
						return tp.DialContext(context.Background(), "tcp", addr.String())
					}
				} else if tp.Dial != nil {
					dial = func(addr net.Addr) (net.Conn, error) {
						return tp.Dial("tcp", addr.String())
					}
				}
				svrCon, err = DialPass(proxy, netil.DomainAddr(svrAddr), dial)
			}
		} else if tp.DialContext != nil {
			svrCon, err = tp.DialContext(context.Background(), "tcp", svrAddr)
		} else if tp.Dial != nil {
			svrCon, err = tp.Dial("tcp", svrAddr)
		} else {
			svrCon, err = net.Dial("tcp", svrAddr)
		}
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}

		cltCon, cltBuf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			svrCon.Close()
			w.WriteHeader(http.StatusBadGateway)
			return
		}

		/*cltConEdr := &gocks5.Eavesdropper{}
		cltConEdr.LimitWriteSpeed(pcc.UpLimiter)
		cltConEdr.LimitReadSpeed(pcc.DownLimiter)
		cltCon = cltConEdr.WrapConn(cltCon) */

		err = netil.WriteAll(cltCon, []byte(r.Proto+" 200 Connection Established\r\n\r\n"))
		if err != nil {
			svrCon.Close()
			cltCon.Close()
			return
		}

		b := netil.AllocBuffer()
		defer netil.RecycleBuffer(b)

		for cltBuf.Reader.Buffered() > 0 {
			n, err := cltBuf.Reader.Read(b)
			if n == 0 {
				svrCon.Close()
				cltCon.Close()
				return
			}
			err = netil.WriteAll(svrCon, b[:n])
			if err != nil {
				svrCon.Close()
				cltCon.Close()
				return
			}
		}

		netil.Forward(cltCon, svrCon, b)
		return
	}

	r.RequestURI = ""
	r.Header.Del("Proxy-Authorization")
	r.Header.Set("Connection", r.Header.Get("Proxy-Connection"))
	r.Header.Del("Proxy-Connection")

	resp, err := tp.RoundTrip(r)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k := range resp.Header {
		for _, v := range resp.Header.Values(k) {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body)
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
