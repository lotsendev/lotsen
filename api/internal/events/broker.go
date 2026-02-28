package events

import "sync"

// StatusEvent carries the deployment ID, its new status, and an optional error message.
type StatusEvent struct {
	DeploymentID string `json:"deploymentId"`
	Status       string `json:"status"`
	Reason       string `json:"reason,omitempty"`
	Error        string `json:"error,omitempty"`
}

// Broker is an in-process pub/sub hub for deployment status events.
// It is safe for concurrent use.
type Broker struct {
	mu   sync.Mutex
	subs map[chan StatusEvent]struct{}
}

// NewBroker creates an empty Broker ready to accept subscribers.
func NewBroker() *Broker {
	return &Broker{subs: make(map[chan StatusEvent]struct{})}
}

// Subscribe registers a new subscriber and returns a read-only event channel
// and a cancel function. The caller must call cancel when done to release
// resources. The channel is buffered; if the subscriber is slow, events are
// dropped rather than blocking the publisher.
func (b *Broker) Subscribe() (<-chan StatusEvent, func()) {
	ch := make(chan StatusEvent, 16)
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

// Publish fans e out to every active subscriber. Subscribers whose buffer is
// full are skipped — the event is dropped for them.
func (b *Broker) Publish(e StatusEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}
