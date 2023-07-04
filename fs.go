package gotor

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

var rawContTypes = map[string]bool{
	"text/html":                     true,
	"text/css":                      true,
	"text/plain":                    true,
	"text/xml":                      true,
	"text/x-component":              true,
	"application/javascript":        true,
	"application/json":              true,
	"application/x-javascript":      true,
	"application/xml":               true,
	"application/xhtml+xml":         true,
	"application/rss+xml":           true,
	"application/atom+xml":          true,
	"application/x-font-ttf":        true,
	"application/vnd.ms-fontobject": true,
	"image/svg+xml":                 true,
	"image/x-icon":                  true,
	"image/bmp":                     true,
	"font/opentype":                 true,
}

func responseFile(w http.ResponseWriter, r *http.Request, f *os.File, fi fs.FileInfo, cacheAge int64, responseName bool) {
	mod := fi.ModTime().Format(http.TimeFormat)

	if r.Header.Get("If-Modified-Since") == mod {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Last-Modified", mod)

	if cacheAge >= 0 {
		w.Header().Set("Cache-Control", "max-age="+strconv.FormatInt(cacheAge, 10))
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}

	fName := fi.Name()
	if responseName && path.Base(r.URL.Path) != fName {
		w.Header().Add("Content-Disposition", "filename=\""+fName+"\"")
	}

	var headBuf []byte

	contType := mime.TypeByExtension(filepath.Ext(fName))
	if contType == "" {
		headBuf = make([]byte, 128)
		n, err := f.Read(headBuf)
		if err != nil {
			NotFound(w, r)
			return
		}
		headBuf = headBuf[:n]
		contType = http.DetectContentType(headBuf)
	}

	w.Header().Set("Content-Type", contType)

	ix := strings.Index(contType, ";")
	if ix == -1 {
		ix = len(contType)
	}

	n := fi.Size()
	if n == 0 {
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusOK)
		return
	}

	if rawContTypes[strings.TrimSpace(contType[:ix])] {
		w.Header().Set("Content-Encoding", "br")
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(n, 10))
	}

	w.WriteHeader(http.StatusOK)

	if headBuf != nil {
		w.Write(headBuf)
	}
	io.Copy(w, f)
}

func ResponseFile(w http.ResponseWriter, r *http.Request, filePath string, cacheAge int64, responseName bool) bool {
	if len(filePath) == 0 {
		return false
	}

	f, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return false
	}

	if fi.IsDir() {
		return false
	}

	responseFile(w, r, f, fi, cacheAge, responseName)
	return true
}

var indexFileNames = []string{
	"index.html",
	"index.htm",
}

func FileService(rootDir string, cacheAge int64, responseName bool, enableIndex bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "..") {
			NotFound(w, r)
			return
		}

		pth := filepath.Join(rootDir, r.URL.Path)

		var f *os.File
		var fi fs.FileInfo
		var err error

		if r.URL.Path[len(r.URL.Path)-1] == '/' {
			if !enableIndex {
				NotFound(w, r)
				return
			}

			for _, indexFileName := range indexFileNames {
				f, err = os.Open(filepath.Join(pth, indexFileName))
				if err == nil {
					break
				}
				f = nil
			}
			if f == nil {
				NotFound(w, r)
				return
			}
			defer f.Close()

			fi, err = f.Stat()
			if err != nil {
				NotFound(w, r)
				return
			}
		} else {
			f, err = os.Open(pth)
			if err != nil {
				NotFound(w, r)
				return
			}
			defer f.Close()

			fi, err = f.Stat()
			if err != nil {
				NotFound(w, r)
				return
			}

			if fi.IsDir() {
				if !enableIndex {
					NotFound(w, r)
					return
				}
				q := r.URL.Query()
				if q.Has("*") {
					q.Del("*")
					r.URL.RawQuery = q.Encode()
				}
				r.URL.Path += "/"
				Redirect(w, r.URL.String(), http.StatusTemporaryRedirect)
				return
			}
		}

		responseFile(w, r, f, fi, cacheAge, responseName)
	}
}
