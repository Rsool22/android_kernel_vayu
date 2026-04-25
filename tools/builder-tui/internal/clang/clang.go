// Package clang fetches and installs LLVM/clang toolchains from upstreams,
// either Google's AOSP gitiles ("main-kernel" branch) or ZyCromerZ/Clang
// GitHub releases. Both implement the Source interface so the caller can pick
// at runtime; an "auto" mode tries Google first and falls back to ZyC.
package clang

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Release describes a downloadable clang asset.
type Release struct {
	Tag       string // human-readable tag/revision
	URL       string // download URL
	AssetName string // file basename
	SizeBytes int64  // -1 if unknown
	Source    string // "google" or "zyc"
}

// Source is implemented by upstream backends.
type Source interface {
	Name() string
	Latest(ctx context.Context, target string) (Release, error)
}

// Auto returns a Source that tries Google first and falls back to ZyC.
type Auto struct {
	HTTP *http.Client
}

func (a *Auto) Name() string { return "auto" }

func (a *Auto) Latest(ctx context.Context, target string) (Release, error) {
	g := &Google{HTTP: a.HTTP}
	r, err := g.Latest(ctx, target)
	if err == nil {
		return r, nil
	}
	z := &ZyC{HTTP: a.HTTP}
	r2, err2 := z.Latest(ctx, "latest")
	if err2 == nil {
		return r2, nil
	}
	return Release{}, fmt.Errorf("auto: google failed (%v); zyc failed (%v)", err, err2)
}

// ─── Google AOSP ──────────────────────────────────────────────────────────────

const googleBase = "https://android.googlesource.com/platform/prebuilts/clang/host/linux-x86"
const googleBranch = "main-kernel"

// Google fetches from the AOSP gitiles JSON API on the main-kernel branch.
type Google struct {
	HTTP *http.Client
	// Branch overrides googleBranch (e.g. "main") when set.
	Branch string
}

func (g *Google) Name() string { return "google" }

func (g *Google) httpClient() *http.Client {
	if g.HTTP != nil {
		return g.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (g *Google) branch() string {
	if g.Branch != "" {
		return g.Branch
	}
	return googleBranch
}

// gitilesEntry mirrors the JSON shape returned by gitiles ?format=JSON.
type gitilesEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type gitilesListing struct {
	Entries []gitilesEntry `json:"entries"`
}

// listRevisions returns clang-r* directory names sorted newest-first.
func (g *Google) listRevisions(ctx context.Context) ([]string, error) {
	url := googleBase + "/+/refs/heads/" + g.branch() + "/?format=JSON"
	body, err := g.fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	body = stripXSSI(body)
	var out gitilesListing
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse gitiles json: %w", err)
	}
	revs := make([]string, 0, len(out.Entries))
	for _, e := range out.Entries {
		if e.Type != "tree" {
			continue
		}
		if !strings.HasPrefix(e.Name, "clang-r") {
			continue
		}
		revs = append(revs, e.Name)
	}
	sort.Slice(revs, func(i, j int) bool {
		return revRank(revs[i]) > revRank(revs[j])
	})
	return revs, nil
}

// revRank converts "clang-r596125b" -> 596125_002 etc. for sorting.
var revPat = regexp.MustCompile(`^clang-r(\d+)([a-z]?)$`)

func revRank(name string) int64 {
	m := revPat.FindStringSubmatch(name)
	if m == nil {
		return 0
	}
	n, _ := strconv.ParseInt(m[1], 10, 64)
	suffix := int64(0)
	if len(m[2]) > 0 {
		suffix = int64(m[2][0] - 'a' + 1)
	}
	return n*1000 + suffix
}

// Latest resolves a Google revision (target="latest" picks the newest).
func (g *Google) Latest(ctx context.Context, target string) (Release, error) {
	revs, err := g.listRevisions(ctx)
	if err != nil {
		return Release{}, err
	}
	if len(revs) == 0 {
		return Release{}, errors.New("google: no clang-r* revisions on branch " + g.branch())
	}
	want := strings.TrimSpace(target)
	if want == "" || want == "latest" {
		want = revs[0]
	} else {
		// Normalize "r596125" -> "clang-r596125".
		if !strings.HasPrefix(want, "clang-") {
			if !strings.HasPrefix(want, "r") {
				want = "r" + want
			}
			want = "clang-" + want
		}
		found := false
		for _, r := range revs {
			if r == want {
				found = true
				break
			}
		}
		if !found {
			return Release{}, fmt.Errorf("google: revision %q not found on branch %s", want, g.branch())
		}
	}
	url := fmt.Sprintf("%s/+archive/refs/heads/%s/%s.tar.gz", googleBase, g.branch(), want)
	return Release{
		Tag:       want,
		URL:       url,
		AssetName: want + ".tar.gz",
		SizeBytes: -1,
		Source:    "google",
	}, nil
}

func (g *Google) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := g.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gitiles: http %d for %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

// stripXSSI removes the ")]}'\n" prefix that gitiles prepends to JSON.
var xssiPrefix = []byte(")]}'\n")

func stripXSSI(b []byte) []byte {
	if len(b) >= len(xssiPrefix) && string(b[:len(xssiPrefix)]) == string(xssiPrefix) {
		return b[len(xssiPrefix):]
	}
	// Some endpoints emit ")]}'" without trailing newline.
	if len(b) >= 4 && string(b[:4]) == ")]}'" {
		i := 4
		for i < len(b) && (b[i] == '\n' || b[i] == '\r') {
			i++
		}
		return b[i:]
	}
	return b
}

// ─── ZyC GitHub Releases ──────────────────────────────────────────────────────

type ZyC struct {
	HTTP *http.Client
	// Token is an optional GitHub PAT to lift API rate limits.
	Token string
}

func (z *ZyC) Name() string { return "zyc" }

func (z *ZyC) httpClient() *http.Client {
	if z.HTTP != nil {
		return z.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Name    string    `json:"name"`
	Assets  []ghAsset `json:"assets"`
}

// Latest resolves a ZyC release matching target ("latest", "23", "15", etc.).
func (z *ZyC) Latest(ctx context.Context, target string) (Release, error) {
	url := "https://api.github.com/repos/ZyCromerZ/Clang/releases?per_page=30"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if z.Token != "" {
		req.Header.Set("Authorization", "Bearer "+z.Token)
	}
	resp, err := z.httpClient().Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 403 {
		return Release{}, errors.New("zyc: github api rate limited")
	}
	if resp.StatusCode != 200 {
		return Release{}, fmt.Errorf("zyc: http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Release{}, err
	}
	var rels []ghRelease
	if err := json.Unmarshal(body, &rels); err != nil {
		return Release{}, fmt.Errorf("parse zyc json: %w", err)
	}
	want := strings.TrimSpace(target)
	if want == "" {
		want = "latest"
	}
	for _, r := range rels {
		if want != "latest" && !strings.Contains(r.TagName, want) {
			continue
		}
		for _, a := range r.Assets {
			if !strings.HasSuffix(a.Name, ".tar.gz") &&
				!strings.HasSuffix(a.Name, ".tar.zst") &&
				!strings.HasSuffix(a.Name, ".tar.xz") {
				continue
			}
			return Release{
				Tag:       r.TagName,
				URL:       a.URL,
				AssetName: a.Name,
				SizeBytes: a.Size,
				Source:    "zyc",
			}, nil
		}
	}
	return Release{}, fmt.Errorf("zyc: no release matched target %q", target)
}

// ─── Install ──────────────────────────────────────────────────────────────────

// ProgressFunc is called periodically with bytes downloaded out of total
// (total <= 0 if unknown).
type ProgressFunc func(downloaded, total int64)

// Install downloads rel into a temp file, then extracts into installDir,
// replacing any existing contents. It verifies that bin/clang is executable
// after extraction.
func Install(ctx context.Context, rel Release, installDir string, progress ProgressFunc) error {
	if installDir == "" {
		return errors.New("install dir is empty")
	}
	tmpFile, err := os.CreateTemp("", "vayu-clang-*"+filepath.Ext(rel.AssetName))
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if err := downloadTo(ctx, rel.URL, tmpFile, progress); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.RemoveAll(installDir); err != nil {
		return fmt.Errorf("clear %s: %w", installDir, err)
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return err
	}
	if err := extract(tmpFile.Name(), installDir); err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	if !hasClang(installDir) {
		return errors.New("extracted archive does not contain bin/clang")
	}
	return nil
}

func hasClang(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "bin", "clang"))
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

func downloadTo(ctx context.Context, url string, w io.Writer, progress ProgressFunc) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	cl := &http.Client{Timeout: 0}
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("http %d for %s", resp.StatusCode, url)
	}
	total := resp.ContentLength
	pr := &progressReader{r: resp.Body, total: total, cb: progress}
	_, err = io.Copy(w, pr)
	return err
}

type progressReader struct {
	r        io.Reader
	read     int64
	total    int64
	cb       ProgressFunc
	lastEmit time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if p.cb != nil && time.Since(p.lastEmit) > 100*time.Millisecond {
		p.cb(p.read, p.total)
		p.lastEmit = time.Now()
	}
	if err != nil && p.cb != nil {
		p.cb(p.read, p.total)
	}
	return n, err
}

func extract(archive, dest string) error {
	// Use system tar; it handles .gz, .xz, .zst (with appropriate util)
	// without us pulling in a Go decompression library for every format.
	cmd := exec.Command("tar", "-xf", archive, "-C", dest)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// LocalVersion runs `clang --version` and parses the version line.
func LocalVersion(installDir string) string {
	bin := filepath.Join(installDir, "bin", "clang")
	if _, err := os.Stat(bin); err != nil {
		return ""
	}
	cmd := exec.Command(bin, "--version")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	first := strings.SplitN(string(out), "\n", 2)[0]
	return strings.TrimSpace(first)
}
