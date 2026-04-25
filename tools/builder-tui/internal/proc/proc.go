// Package proc wraps os/exec with patterns we use everywhere: capture stdout,
// stream lines to a callback, and return both exit code and combined output.
package proc

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// LineFunc receives a single line of stdout/stderr while the process runs.
type LineFunc func(line string, isStderr bool)

// Result captures the outcome of a Run call.
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Combined string
	Err      error
}

// Ok reports success (exit 0, no error).
func (r Result) Ok() bool { return r.Err == nil && r.ExitCode == 0 }

// Run executes a command with optional environment, working dir, and per-line
// streaming. The command is *not* run through a shell.
func Run(ctx context.Context, dir string, env []string, line LineFunc, name string, args ...string) Result {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return Result{Err: err}
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return Result{Err: err}
	}

	if err := cmd.Start(); err != nil {
		return Result{Err: err}
	}

	var stdoutBuf, stderrBuf, combined strings.Builder
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(2)

	consume := func(r io.Reader, isStderr bool, sink *strings.Builder) {
		defer wg.Done()
		s := bufio.NewScanner(r)
		s.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		for s.Scan() {
			ln := s.Text()
			mu.Lock()
			sink.WriteString(ln)
			sink.WriteByte('\n')
			combined.WriteString(ln)
			combined.WriteByte('\n')
			mu.Unlock()
			if line != nil {
				line(ln, isStderr)
			}
		}
	}
	go consume(stdoutPipe, false, &stdoutBuf)
	go consume(stderrPipe, true, &stderrBuf)
	wg.Wait()

	err = cmd.Wait()
	res := Result{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Combined: combined.String(),
	}
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.ExitCode = ee.ExitCode()
		} else {
			res.Err = err
		}
	}
	return res
}

// Capture is a convenience for short, fast commands where streaming is
// unnecessary (e.g. `git ls-remote`, `clang --version`).
func Capture(ctx context.Context, name string, args ...string) Result {
	return Run(ctx, "", nil, nil, name, args...)
}
