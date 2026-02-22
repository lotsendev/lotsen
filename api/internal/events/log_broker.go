package events

import "sync"

const defaultLogBacklog = 100

// LogLine carries a single log line from a deployment container.
type LogLine struct {
	Line string `json:"line"`
}

// LogBroker buffers the last N log lines per deployment and fans new lines
// out to all active subscribers. It is safe for concurrent use.
type LogBroker struct {
	mu   sync.Mutex
	bufs map[string]*ringBuf
	subs map[string][]chan LogLine
}

// NewLogBroker creates an empty LogBroker.
func NewLogBroker() *LogBroker {
	return &LogBroker{
		bufs: make(map[string]*ringBuf),
		subs: make(map[string][]chan LogLine),
	}
}

// Append stores line in the ring buffer for deploymentID and broadcasts it to
// all active subscribers. Subscribers whose buffer is full are skipped.
func (b *LogBroker) Append(deploymentID, line string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	buf, ok := b.bufs[deploymentID]
	if !ok {
		buf = newRingBuf(defaultLogBacklog)
		b.bufs[deploymentID] = buf
	}
	buf.push(line)

	ll := LogLine{Line: line}
	for _, ch := range b.subs[deploymentID] {
		select {
		case ch <- ll:
		default:
		}
	}
}

// Subscribe returns the current backlog for deploymentID and a live channel
// for subsequent lines. The caller must invoke cancel when done to release
// resources.
func (b *LogBroker) Subscribe(deploymentID string) (backlog []LogLine, ch <-chan LogLine, cancel func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if buf := b.bufs[deploymentID]; buf != nil {
		for _, line := range buf.lines() {
			backlog = append(backlog, LogLine{Line: line})
		}
	}

	liveCh := make(chan LogLine, 16)
	b.subs[deploymentID] = append(b.subs[deploymentID], liveCh)

	cancel = func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.subs[deploymentID]
		for i, s := range subs {
			if s == liveCh {
				b.subs[deploymentID] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}
	return backlog, liveCh, cancel
}

// ringBuf is a fixed-capacity circular buffer for strings.
type ringBuf struct {
	data []string
	head int
	size int
	cap  int
}

func newRingBuf(capacity int) *ringBuf {
	return &ringBuf{data: make([]string, capacity), cap: capacity}
}

func (r *ringBuf) push(s string) {
	r.data[r.head%r.cap] = s
	r.head++
	if r.size < r.cap {
		r.size++
	}
}

// lines returns buffered lines in insertion order (oldest first).
func (r *ringBuf) lines() []string {
	if r.size == 0 {
		return nil
	}
	out := make([]string, r.size)
	start := r.head - r.size
	for i := 0; i < r.size; i++ {
		out[i] = r.data[(start+i)%r.cap]
	}
	return out
}
