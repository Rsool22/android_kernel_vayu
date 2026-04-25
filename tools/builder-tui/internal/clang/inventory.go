package clang

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry describes one installed clang directory inside a library tree.
//
// The inventory model treats `<libDir>/clang` as the *active* slot that
// the build pipeline points at, and `<libDir>/<source>-<tag>` directories
// as the available versions. Switching between versions is done by
// updating the `clang` symlink atomically.
type Entry struct {
	// Dir is the absolute path to the install directory. It contains
	// at least bin/clang.
	Dir string
	// Tag is a human-readable label derived from the directory name
	// ("google-r530567" or "zyc-19.1.6").
	Tag string
	// Source is "google", "zyc", "auto", or "" when unrecognised.
	Source string
	// Active is true when this entry is the currently-selected version
	// (i.e. <libDir>/clang resolves to this directory).
	Active bool
}

// LibraryDir returns the directory which holds the multi-version
// inventory. It's the parent directory of `clangDir` (which is the
// active slot, e.g. `~/toolchains/clang`). Falls back to clangDir's
// parent if it can't be derived.
func LibraryDir(clangDir string) string {
	if clangDir == "" {
		return ""
	}
	return filepath.Dir(clangDir)
}

// Inventory returns all clang installs under libDir, plus the entry
// that <libDir>/clang currently resolves to (if any). Inactive entries
// without a bin/clang are skipped silently.
func Inventory(libDir, activeSlot string) ([]Entry, error) {
	if libDir == "" {
		return nil, errors.New("inventory: libDir is empty")
	}
	entries, err := os.ReadDir(libDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", libDir, err)
	}

	// Resolve active slot to a real directory once; entries equal to
	// it (after symlink resolution) are tagged Active.
	var activeReal string
	if activeSlot != "" {
		if rp, err := filepath.EvalSymlinks(activeSlot); err == nil {
			activeReal = rp
		}
	}

	var out []Entry
	for _, e := range entries {
		full := filepath.Join(libDir, e.Name())
		// Skip the active slot itself; it's represented by the
		// real directory it points at.
		if full == activeSlot {
			continue
		}
		st, err := os.Stat(full)
		if err != nil || !st.IsDir() {
			continue
		}
		if !hasClang(full) {
			continue
		}
		realDir := full
		if rp, err := filepath.EvalSymlinks(full); err == nil {
			realDir = rp
		}
		ent := Entry{
			Dir:    full,
			Tag:    e.Name(),
			Source: deriveSource(e.Name()),
			Active: realDir == activeReal && activeReal != "",
		}
		out = append(out, ent)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tag < out[j].Tag })
	return out, nil
}

// deriveSource extracts the leading "<source>-" prefix from the
// directory name. Returns "" when the prefix is not recognised.
func deriveSource(name string) string {
	for _, p := range []string{"google", "zyc", "auto"} {
		if strings.HasPrefix(name, p+"-") || name == p {
			return p
		}
	}
	return ""
}

// VersionedDirName returns the canonical inventory directory for a
// release, e.g. "google-r530567" or "zyc-19.1.6". Falls back to the
// source name alone when the tag is empty.
func VersionedDirName(rel Release) string {
	src := rel.Source
	if src == "" {
		src = "clang"
	}
	tag := strings.TrimSpace(rel.Tag)
	if tag == "" {
		return src
	}
	// Sanitise: strip whitespace and slashes that would break the
	// filesystem layout.
	tag = strings.ReplaceAll(tag, "/", "-")
	tag = strings.ReplaceAll(tag, " ", "-")
	return src + "-" + tag
}

// InstallVersioned downloads rel into <libDir>/<source>-<tag>/ and
// returns the absolute install directory. Existing installs at the same
// versioned path are replaced. The active slot (libDir/clang) is left
// untouched -- callers must invoke MakeActive to switch.
func InstallVersioned(ctx context.Context, rel Release, libDir string, progress ProgressFunc) (string, error) {
	if libDir == "" {
		return "", errors.New("InstallVersioned: libDir is empty")
	}
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(libDir, VersionedDirName(rel))
	if err := Install(ctx, rel, dst, progress); err != nil {
		return "", err
	}
	return dst, nil
}

// MakeActive points <libDir>/clang at the directory `target` (which
// must live under libDir) by replacing the slot with a symlink. When
// the slot is currently a real directory, it is moved aside to the
// inventory before the swap so the previous version is preserved.
func MakeActive(libDir, target string) error {
	if libDir == "" || target == "" {
		return errors.New("MakeActive: libDir and target must be non-empty")
	}
	slot := filepath.Join(libDir, "clang")

	// If the slot already resolves to target, nothing to do.
	if rp, err := filepath.EvalSymlinks(slot); err == nil && rp == target {
		return nil
	}

	// If the slot exists as a *real* directory (not a symlink), move
	// it into the inventory under a unique name so it isn't lost.
	if st, err := os.Lstat(slot); err == nil {
		if st.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(slot); err != nil {
				return fmt.Errorf("remove slot symlink: %w", err)
			}
		} else {
			parked := slot + ".orphan"
			for i := 1; ; i++ {
				if _, err := os.Stat(parked); os.IsNotExist(err) {
					break
				}
				parked = fmt.Sprintf("%s.orphan-%d", slot, i)
			}
			if err := os.Rename(slot, parked); err != nil {
				return fmt.Errorf("park existing slot: %w", err)
			}
		}
	}

	if err := os.Symlink(target, slot); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", slot, target, err)
	}
	return nil
}

// Remove deletes a versioned install. Refuses to delete the active
// slot or a directory the active slot is currently pointing at.
func Remove(libDir, target string) error {
	if libDir == "" || target == "" {
		return errors.New("Remove: libDir and target must be non-empty")
	}
	slot := filepath.Join(libDir, "clang")
	if rp, err := filepath.EvalSymlinks(slot); err == nil && rp == target {
		return errors.New("refusing to delete the active clang -- switch first")
	}
	if filepath.Clean(target) == filepath.Clean(slot) {
		return errors.New("refusing to delete the active slot directly")
	}
	return os.RemoveAll(target)
}
