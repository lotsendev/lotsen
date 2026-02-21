package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
)

const addr = ":8080"

func main() {
	mux := http.NewServeMux()

	// API routes — registered here so later slices can extend the mux.
	mux.HandleFunc("/api/", apiNotFound)

	// All other requests are handled by the SPA: serve a file from
	// gui/dist if it exists, otherwise fall back to index.html so
	// the React router can handle client-side navigation.
	mux.Handle("/", spaHandler("gui/dist"))

	log.Printf("dirigent listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("dirigent: %v", err)
	}
}

// apiNotFound is a placeholder until real API handlers are registered.
func apiNotFound(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not found", http.StatusNotFound)
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
