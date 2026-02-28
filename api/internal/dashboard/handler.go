package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:static
var staticAssets embed.FS

func New(apiHandler http.Handler) http.Handler {
	assets, err := fs.Sub(staticAssets, "static")
	if err != nil {
		panic(err)
	}

	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		index, err = fs.ReadFile(assets, "zz-placeholder.html")
		if err != nil {
			panic(err)
		}
	}

	fileServer := http.FileServer(http.FS(assets))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPIPath(r.URL.Path) {
			apiHandler.ServeHTTP(w, r)
			return
		}

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		assetPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if assetPath == "." || assetPath == "" {
			serveIndex(w, r, index)
			return
		}

		if assetPath == "index.html" || fileExists(assets, assetPath) {
			fileServer.ServeHTTP(w, r)
			return
		}

		serveIndex(w, r, index)
	})
}

func isAPIPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

func fileExists(fsys fs.FS, name string) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return false
	}

	return !info.IsDir()
}

func serveIndex(w http.ResponseWriter, r *http.Request, body []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write(body)
}
