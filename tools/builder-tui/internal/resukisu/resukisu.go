// Package resukisu provides ReSukiSU branch probing, install, and update.
// The probe distinguishes "branch absent" (network ok, ref missing) from
// "network failure" (no connectivity), which is needed by both the TUI and CI.
package resukisu

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Repo is the upstream ReSukiSU repo.
const Repo = "https://github.com/ReSukiSU/ReSukiSU.git"

// State enumerates the result of a branch probe.
type State string

const (
	StatePresent     State = "present"
	StateAbsent      State = "absent"
	StateNetworkFail State = "network_fail"
)

// Probe contains the result of probing a single branch.
type Probe struct {
	Branch string
	State  State
	SHA    string // populated when State == StatePresent
	Err    error  // raw error when network_fail
}

// ProbeBranch checks whether ReSukiSU has a given branch upstream. It uses
// `git ls-remote --exit-code` so we get distinct exit codes for "ref missing"
// (2) vs "network down" (128).
func ProbeBranch(ctx context.Context, branch string) Probe {
	p := Probe{Branch: branch}
	cmd := exec.CommandContext(ctx, "git",
		"ls-remote", "--exit-code", Repo, "refs/heads/"+branch)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	switch {
	case err == nil:
		s := strings.TrimSpace(out.String())
		// First field of the first line is the SHA.
		fields := strings.Fields(strings.SplitN(s, "\n", 2)[0])
		if len(fields) > 0 && len(fields[0]) >= 7 {
			p.State = StatePresent
			p.SHA = fields[0]
			return p
		}
		p.State = StateAbsent
		return p
	default:
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 2 {
			p.State = StateAbsent
			return p
		}
		// Sanity-probe main; if main also fails, this is networking.
		if branch != "main" {
			ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			mctx := exec.CommandContext(ctx2, "git",
				"ls-remote", "--exit-code", Repo, "refs/heads/main")
			mctx.Stdout = &bytes.Buffer{}
			mctx.Stderr = &bytes.Buffer{}
			if mctx.Run() == nil {
				p.State = StateAbsent
				return p
			}
		}
		p.State = StateNetworkFail
		p.Err = errors.New(strings.TrimSpace(errBuf.String()))
		return p
	}
}

// ProbeAll runs both main and dev probes concurrently.
func ProbeAll(ctx context.Context) (main, dev Probe) {
	type r struct {
		name string
		p    Probe
	}
	ch := make(chan r, 2)
	go func() { ch <- r{"main", ProbeBranch(ctx, "main")} }()
	go func() { ch <- r{"dev", ProbeBranch(ctx, "dev")} }()
	for i := 0; i < 2; i++ {
		v := <-ch
		switch v.name {
		case "main":
			main = v.p
		case "dev":
			dev = v.p
		}
	}
	return
}

// LocalCommit returns the SHA of the local drivers/kernelsu install,
// walking up to the nearest .git directory if the kernelsu dir is a
// submodule or symlink. Empty string if not installed.
func LocalCommit(kernelDir string) string {
	dir := filepath.Join(kernelDir, "drivers", "kernelsu")
	if _, err := exec.LookPath("git"); err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		resolved = dir
	}
	cur := resolved
	for cur != "/" && cur != "" {
		if dirHasGit(cur) {
			out, err := exec.Command("git", "-C", cur, "rev-parse", "HEAD").Output()
			if err != nil {
				return ""
			}
			return strings.TrimSpace(string(out))
		}
		cur = filepath.Dir(cur)
	}
	return ""
}

func dirHasGit(d string) bool {
	cmd := exec.Command("git", "-C", d, "rev-parse", "--is-inside-work-tree")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// Cleanup runs ReSukiSU's official setup.sh --cleanup, then sweeps any
// stale kernelsu artefacts the script may not delete (KernelSU/ clone,
// out/drivers/kernelsu/, dangling drivers/kernelsu/kernel symlink).
//
// Returns the cleanup script exit code (0 == success), the .c file count
// remaining (sanity check), and any wrapper error from launching bash.
func Cleanup(ctx context.Context, kernelDir, outputDir string, line func(string)) (rc int, ksuC int, err error) {
	url := "https://raw.githubusercontent.com/ReSukiSU/ReSukiSU/main/kernel/setup.sh"
	resp, herr := (&http.Client{Timeout: 30 * time.Second}).Get(url)
	if herr != nil {
		return -1, -1, herr
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return -1, -1, fmt.Errorf("fetch setup.sh: http %d", resp.StatusCode)
	}
	cmd := exec.CommandContext(ctx, "bash", "-s", "--cleanup")
	cmd.Stdin = resp.Body
	cmd.Dir = kernelDir
	pipe, perr := cmd.StdoutPipe()
	if perr != nil {
		return -1, -1, perr
	}
	cmd.Stderr = cmd.Stdout
	if serr := cmd.Start(); serr != nil {
		return -1, -1, serr
	}
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		if line != nil {
			line(scanner.Text())
		}
	}
	werr := cmd.Wait()
	rc = 0
	if werr != nil {
		var ee *exec.ExitError
		if errors.As(werr, &ee) {
			rc = ee.ExitCode()
		} else {
			rc = -1
		}
	}
	if d := filepath.Join(kernelDir, "KernelSU"); dirExists(d) {
		_ = os.RemoveAll(d)
	}
	if outputDir != "" {
		if d := filepath.Join(outputDir, "drivers", "kernelsu"); dirExists(d) {
			_ = os.RemoveAll(d)
		}
	}
	intLink := filepath.Join(kernelDir, "drivers", "kernelsu", "kernel")
	if isDanglingSymlink(intLink) {
		_ = os.Remove(intLink)
	}
	ksuC = countSourceFiles(filepath.Join(kernelDir, "drivers", "kernelsu"))
	return rc, ksuC, nil
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func isDanglingSymlink(p string) bool {
	li, err := os.Lstat(p)
	if err != nil || li.Mode()&os.ModeSymlink == 0 {
		return false
	}
	_, err = os.Stat(p)
	return err != nil
}

func countSourceFiles(root string) int {
	n := 0
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".c") {
			n++
		}
		return nil
	})
	return n
}

// Install runs ReSukiSU's official setup.sh installer for the given branch.
// kernelDir is the kernel source root (cwd for the install script).
func Install(ctx context.Context, kernelDir, branch string, line func(string)) error {
	url := fmt.Sprintf("https://raw.githubusercontent.com/ReSukiSU/ReSukiSU/%s/kernel/setup.sh", branch)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("fetch setup.sh: http %d", resp.StatusCode)
	}
	cmd := exec.CommandContext(ctx, "bash", "-s", branch)
	cmd.Stdin = resp.Body
	cmd.Dir = kernelDir
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		if line != nil {
			line(scanner.Text())
		}
	}
	return cmd.Wait()
}
