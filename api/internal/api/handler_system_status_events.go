package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type systemStatusBroker struct {
	mu   sync.Mutex
	subs map[chan SystemStatusSnapshot]struct{}
}

func newSystemStatusBroker() *systemStatusBroker {
	return &systemStatusBroker{subs: make(map[chan SystemStatusSnapshot]struct{})}
}

func (b *systemStatusBroker) Subscribe() (<-chan SystemStatusSnapshot, func()) {
	ch := make(chan SystemStatusSnapshot, 16)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
	}

	return ch, cancel
}

func (b *systemStatusBroker) Publish(snapshot SystemStatusSnapshot) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for ch := range b.subs {
		select {
		case ch <- snapshot:
		default:
		}
	}
}

func (h *Handler) systemStatusEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, cancel := h.statusEvents.Subscribe()
	defer cancel()

	write := func(snapshot SystemStatusSnapshot) error {
		data, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	if err := write(h.currentSystemStatusSnapshot(r)); err != nil {
		log.Printf("systemStatusEvents: write initial snapshot: %v", err)
		return
	}

	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			if _, err := w.Write([]byte(": keepalive\n\n")); err != nil {
				return
			}
			flusher.Flush()
		case snapshot := <-ch:
			if err := write(snapshot); err != nil {
				log.Printf("systemStatusEvents: write snapshot: %v", err)
				return
			}
		}
	}
}
