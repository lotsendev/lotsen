package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ercadev/dirigent/proxy/internal/handler"
	"github.com/ercadev/dirigent/proxy/internal/poller"
	"github.com/ercadev/dirigent/proxy/internal/routing"
	"github.com/ercadev/dirigent/store"
)

const defaultAddr = ":80"

func dataPath() string {
	if p := os.Getenv("DIRIGENT_DATA"); p != "" {
		return p
	}
	return "/var/lib/dirigent/deployments.json"
}

func main() {
	s, err := store.NewJSONStore(dataPath())
	if err != nil {
		log.Fatalf("proxy: open store: %v", err)
	}

	table := routing.NewTable()

	interval := 5 * time.Second
	if v := os.Getenv("DIRIGENT_POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		}
	}

	p := poller.New(s, table, interval)

	mux := http.NewServeMux()
	handler.New(table).RegisterRoutes(mux)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go p.Run(ctx)

	addr := defaultAddr
	if v := os.Getenv("DIRIGENT_PROXY_ADDR"); v != "" {
		addr = v
	}

	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		<-ctx.Done()
		if err := srv.Close(); err != nil {
			log.Printf("proxy: shutdown: %v", err)
		}
	}()

	log.Printf("proxy: listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("proxy: %v", err)
	}
}
