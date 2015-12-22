package gotor

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

type FileService struct{
	CacheAge int64
	Encodings map[string]string
}

var EncodingsWebResources = map[string]string{
	"text/html": "gzip",
	"text/css": "gzip",
	"text/plain": "gzip",
	"application/javascript": "gzip",
	"image/bmp": "gzip",
}

var encoder = map[string]func(io.Writer)io.WriteCloser{
	"gzip": func(w io.Writer)io.WriteCloser{return gzip.NewWriter(w)},
}

func (fs *FileService) Single(w http.ResponseWriter, r *http.Request, fileName string) {
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
	}
	w.Header().Set("Last-Modified", mod)
	w.Header().Set("Cache-Control", "max-age=" + strconv.FormatInt(fs.CacheAge, 10))

	fName := fi.Name()
	if path.Base(r.URL.Path) != fName {
		w.Header().Add("Content-Disposition", "filename=\"" + fName + "\"")
	}

	bin := make([]byte, 32768)
	count, err := f.Read(bin)

	contType := mime.TypeByExtension(filepath.Ext(fName))
	if contType == "" {
		contType = http.DetectContentType(bin)
	}

	w.Header().Set("Content-Type", contType)

	ix := strings.Index(contType, ";")
	if ix == -1 {
		ix = len(contType)
	}

	var encoType string
	var ok bool
	if fs.Encodings != nil {
		encoType, ok = fs.Encodings[strings.TrimSpace(contType[:ix])]
	}

	if ok && strings.Index(r.Header.Get("Accept-Encoding"), encoType) != -1 {
		w.Header().Set("Content-Encoding", encoType)
		if fi.Size() <= 32768 {
			buf := bytes.NewBuffer(make([]byte, 0, count))
			z := encoder[encoType](buf)
			z.Write(bin[:count])
			z.Close()
			w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
			w.WriteHeader(http.StatusOK)
			w.Write(buf.Bytes())
		}else{
			w.WriteHeader(http.StatusOK)
			z := encoder[encoType](w)
			z.Write(bin[:count])
			if err != io.EOF {
				io.CopyBuffer(z, f, bin)
			}
			z.Close()
		}
		return
	}

	w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	w.WriteHeader(http.StatusOK)
	w.Write(bin[:count])
	if err != io.EOF {
		io.CopyBuffer(w, f, bin)
	}
}

func (fs *FileService) BindDir(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fs.Single(w, r, filepath.Join(root, r.URL.Query().Get("*")))
	}
}
