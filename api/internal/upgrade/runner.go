package upgrade

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type State string

const (
	StateIdle    State = "idle"
	StateRunning State = "running"
	StateDone    State = "done"
	StateFailed  State = "failed"
)

const defaultLogPath = "/tmp/lotsen-upgrade.log"

var (
	ErrAlreadyRunning = errors.New("upgrade already running")
	ErrNotRunning     = errors.New("upgrade is not running")
)

type processBuilder func(targetVersion string) (processConfig, error)

type processConfig struct {
	path    string
	args    []string
	env     []string
	cleanup func()
}

type Runner struct {
	mu          sync.Mutex
	state       State
	logPath     string
	build       processBuilder
	subscribers map[int]chan string
	nextID      int
}

func New() *Runner {
	return NewWithBuilder(defaultLogPath, defaultProcessBuilder)
}

func NewWithBuilder(logPath string, builder processBuilder) *Runner {
	if logPath == "" {
		logPath = defaultLogPath
	}
	if builder == nil {
		builder = defaultProcessBuilder
	}

	return &Runner{state: StateIdle, logPath: logPath, build: builder, subscribers: make(map[int]chan string)}
}

func (r *Runner) Start(targetVersion string) error {
	r.mu.Lock()
	if r.state == StateRunning {
		r.mu.Unlock()
		return ErrAlreadyRunning
	}
	r.state = StateRunning
	r.mu.Unlock()

	cfg, err := r.build(targetVersion)
	if err != nil {
		r.finish(StateFailed)
		return err
	}

	stdin, err := os.Open("/dev/null")
	if err != nil {
		r.finish(StateFailed)
		cfg.cleanup()
		return fmt.Errorf("open /dev/null: %w", err)
	}

	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		stdin.Close()
		r.finish(StateFailed)
		cfg.cleanup()
		return fmt.Errorf("create output pipe: %w", err)
	}

	logFile, err := os.OpenFile(r.logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		stdin.Close()
		pipeReader.Close()
		pipeWriter.Close()
		r.finish(StateFailed)
		cfg.cleanup()
		return fmt.Errorf("open upgrade log file: %w", err)
	}

	proc, err := os.StartProcess(cfg.path, cfg.args, &os.ProcAttr{
		Env:   cfg.env,
		Files: []*os.File{stdin, pipeWriter, pipeWriter},
		Sys:   &syscall.SysProcAttr{Setsid: true},
	})
	stdin.Close()
	pipeWriter.Close()
	if err != nil {
		pipeReader.Close()
		logFile.Close()
		r.finish(StateFailed)
		cfg.cleanup()
		return fmt.Errorf("start upgrade process: %w", err)
	}

	go r.collect(proc, pipeReader, logFile, cfg.cleanup)
	return nil
}

func (r *Runner) Subscribe() (<-chan string, func(), error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state != StateRunning {
		return nil, nil, ErrNotRunning
	}

	id := r.nextID
	r.nextID++
	ch := make(chan string, 32)
	r.subscribers[id] = ch

	unsubscribe := func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		sub, ok := r.subscribers[id]
		if !ok {
			return
		}
		delete(r.subscribers, id)
		close(sub)
	}

	return ch, unsubscribe, nil
}

func (r *Runner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state == StateRunning
}

func (r *Runner) collect(proc *os.Process, pipeReader, logFile *os.File, cleanup func()) {
	defer pipeReader.Close()
	defer logFile.Close()
	defer cleanup()

	scanner := bufio.NewScanner(pipeReader)
	for scanner.Scan() {
		line := scanner.Text()
		_, _ = logFile.WriteString(line + "\n")
		r.broadcast(line)
	}

	state := StateDone
	if err := scanner.Err(); err != nil {
		state = StateFailed
	}

	ps, err := proc.Wait()
	if err != nil || !ps.Success() {
		state = StateFailed
	}

	r.finish(state)
}

func (r *Runner) broadcast(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, ch := range r.subscribers {
		select {
		case ch <- line:
		default:
			delete(r.subscribers, id)
			close(ch)
		}
	}
}

func (r *Runner) finish(state State) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = state
	for id, ch := range r.subscribers {
		delete(r.subscribers, id)
		close(ch)
	}
}

func defaultProcessBuilder(targetVersion string) (processConfig, error) {
	if targetVersion == "" {
		targetVersion = "latest"
	}

	startedAt := time.Now().UTC().Format(time.RFC3339)

	if systemdRunPath, err := exec.LookPath("systemd-run"); err == nil {
		unit := fmt.Sprintf("lotsen-upgrade-%d", time.Now().UnixNano())
		return processConfig{
			path: systemdRunPath,
			args: []string{
				"systemd-run",
				"--unit", unit,
				"--collect",
				"--no-block",
				"--setenv=LOTSEN_UPGRADE_STARTED_AT=" + startedAt,
				"/bin/sh",
				"-c",
				"exec /usr/local/bin/lotsen upgrade --to \"$1\" --non-interactive --yes >> /tmp/lotsen-upgrade.log 2>&1",
				"sh",
				targetVersion,
			},
			env: os.Environ(),
			cleanup: func() {
			},
		}, nil
	}

	path := "/usr/local/bin/lotsen"
	args := []string{"lotsen", "upgrade", "--to", targetVersion, "--non-interactive", "--yes"}
	env := append(os.Environ(), "LOTSEN_UPGRADE_STARTED_AT="+startedAt)

	return processConfig{
		path:    path,
		args:    args,
		env:     env,
		cleanup: func() {},
	}, nil
}
