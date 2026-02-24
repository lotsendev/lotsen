package upgrade

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStart_DoubleStartReturnsError(t *testing.T) {
	scriptPath := writeScript(t, "#!/usr/bin/env bash\necho one\nsleep 0.2\necho two\n")
	build := func(_ string) (processConfig, error) {
		return processConfig{
			path: "/bin/bash",
			args: []string{"bash", scriptPath},
			env:  os.Environ(),
			cleanup: func() {
			},
		}, nil
	}

	r := NewWithBuilder(filepath.Join(t.TempDir(), "upgrade.log"), build)

	if err := r.Start("v1.2.3"); err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	if err := r.Start("v1.2.3"); err == nil {
		t.Fatal("second Start() error = nil, want error")
	}
}

func TestStart_LogLinesReadableFromChannel(t *testing.T) {
	scriptPath := writeScript(t, "#!/usr/bin/env bash\necho hello\necho world\n")
	build := func(_ string) (processConfig, error) {
		return processConfig{
			path: "/bin/bash",
			args: []string{"bash", scriptPath},
			env:  os.Environ(),
			cleanup: func() {
			},
		}, nil
	}

	r := NewWithBuilder(filepath.Join(t.TempDir(), "upgrade.log"), build)
	if err := r.Start("v1.2.3"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	ch, unsubscribe, err := r.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	defer unsubscribe()

	lines := make([]string, 0, 2)
	timeout := time.After(3 * time.Second)
	for len(lines) < 2 {
		select {
		case line, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed before receiving all lines: got %v", lines)
			}
			lines = append(lines, line)
		case <-timeout:
			t.Fatalf("timed out waiting for lines, got %v", lines)
		}
	}

	if lines[0] != "hello" || lines[1] != "world" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func writeScript(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "dummy.sh")
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	return path
}
