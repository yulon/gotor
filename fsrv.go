package gotor

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

type FileService struct {
	CacheAge int64
	Encodings map[string]string
	ResponseFileName bool
}

var EncodingsWebResources = map[string]string{
	"text/html": "gzip",
	"text/css": "gzip",
	"text/plain": "gzip",
	"application/javascript": "gzip",
	"application/json": "gzip",
	"image/bmp": "gzip",
}

func (fs *FileService) Single(w http.ResponseWriter, r *http.Request, fileName string) {
	if fileName == "" || fileName == "" {
		NotFound(w, r)
		return
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
	w.Header().Set("Cache-Control", "max-age="+strconv.FormatInt(fs.CacheAge, 10))

	fName := fi.Name()
	if fs.ResponseFileName && path.Base(r.URL.Path) != fName {
		w.Header().Add("Content-Disposition", "filename=\""+fName+"\"")
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
	ok := false
	if fs.Encodings != nil {
		encoType, ok = fs.Encodings[strings.TrimSpace(contType[:ix])]
	}

	if ok {
		w.Header().Set("Content-Encoding", encoType)
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	}

	w.WriteHeader(http.StatusOK)
	w.Write(bin[:count])
	if err != io.EOF {
		io.CopyBuffer(w, f, bin)
	}
}

func (fs *FileService) Bridge(rootDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fs.Single(w, r, filepath.Join(rootDir, r.URL.Query().Get("*")))
	}
}
