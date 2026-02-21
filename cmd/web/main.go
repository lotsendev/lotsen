package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
)

const addr = ":3000"

func main() {
	mux := http.NewServeMux()

	// Serve the React production build. Any path that does not map to an
	// existing file falls back to index.html so the React router can handle
	// client-side navigation.
	mux.Handle("/", spaHandler("gui/dist"))

	log.Printf("dirigent web listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("web: %v", err)
	}
}

// spaHandler returns an http.Handler that serves static files from dir.
// Any request whose path does not map to an existing file is served as
// index.html, allowing the React router to handle client-side routes.
func spaHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		info, err := os.Stat(path)
		if os.IsNotExist(err) || (err == nil && info.IsDir()) {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})
}
