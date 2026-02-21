package main

import (
	"log"
	"net/http"
	"os"

	"github.com/ercadev/dirigent/internal/api"
	"github.com/ercadev/dirigent/store"
)

const addr = ":8080"

func dataPath() string {
	if p := os.Getenv("DIRIGENT_DATA"); p != "" {
		return p
	}
	return "/var/lib/dirigent/deployments.json"
}

func main() {
	s, err := store.NewJSONStore(dataPath())
	if err != nil {
		log.Fatalf("dirigent: open store: %v", err)
	}

	mux := http.NewServeMux()
	api.New(s).RegisterRoutes(mux)

	log.Printf("dirigent API listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("dirigent: %v", err)
	}
}
