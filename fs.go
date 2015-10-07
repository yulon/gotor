package hs

import (
	"os"
	"net/http"
	"strings"
	"compress/gzip"
	"bytes"
	"io"
	"strconv"
	"path"
	"path/filepath"
	"mime"
)

var PreGzipFileMimes = map[string]bool{
	"text/html": true,
	"text/css": true,
	"text/plain": true,
	"application/javascript": true,
	"image/bmp": true,
}

func HandleFile(w http.ResponseWriter, r *http.Request, fileName string) {
	if fileName == "" || fileName == "" {
		NotFound(w, r)
	}

	f, err := os.Open(fileName)
	if err != nil {
		NotFound(w, r)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		NotFound(w, r)
		return
	}

	mod := fi.ModTime().Format(http.TimeFormat)

	if r.Header.Get("Cache-Control") != "no-cache" && r.Header.Get("If-Modified-Since") == mod {
		w.WriteHeader(http.StatusNotModified)
		return
	}else{
		w.Header().Set("Last-Modified", mod)
	}

	fName := fi.Name()
	if path.Base(r.URL.Path) != fName {
		w.Header().Add("Content-Disposition", "filename=\"" + fName + "\"")
	}

	bin := make([]byte, 32768)
	count, err := f.Read(bin)

	cType := mime.TypeByExtension(filepath.Ext(fName))
	if cType == "" {
		cType = http.DetectContentType(bin)
	}

	w.Header().Set("Content-Type", cType)

	if PreGzipFileMimes[strings.Split(cType, ";")[0]] == true {
		ae := strings.Split(r.Header.Get("Accept-Encoding"), ",")
		for i := 0; i < len(ae); i++ {
			if strings.TrimSpace(ae[i]) == "gzip" {
				w.Header().Set("Content-Encoding", "gzip")
				if fi.Size() <= 32768 {
					buf := bytes.NewBuffer(make([]byte, 0, count))
					z := gzip.NewWriter(buf)
					z.Write(bin[:count])
					z.Close()
					w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
					w.WriteHeader(http.StatusOK)
					w.Write(buf.Bytes())
				}else{
					w.WriteHeader(http.StatusOK)
					z := gzip.NewWriter(w)
					z.Write(bin[:count])
					if err != io.EOF {
						io.CopyBuffer(z, f, bin)
					}
					z.Close()
				}
				return
			}
		}
	}

	w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	w.WriteHeader(http.StatusOK)
	w.Write(bin[:count])
	if err != io.EOF {
		io.CopyBuffer(w, f, bin)
	}
}

func FileHandler(root string) Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		HandleFile(w, r, filepath.Join(root, r.URL.Query().Get("@rest")))
	}
}
