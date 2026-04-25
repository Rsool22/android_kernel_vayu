// Package pkg3 packages a built kernel image into an AnyKernel3 zip.
package pkg3

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/proc"
)

// Package copies the kernel image into anyKernelDir and zips the directory
// into outputZip. Any pre-existing Image.gz-dtb in anyKernelDir is replaced.
func Package(ctx context.Context, anyKernelDir, kernelImage, outputZip string) error {
	if !dirExists(anyKernelDir) {
		return fmt.Errorf("anykernel dir missing: %s", anyKernelDir)
	}
	if _, err := os.Stat(kernelImage); err != nil {
		return fmt.Errorf("kernel image missing: %s", kernelImage)
	}
	dest := filepath.Join(anyKernelDir, filepath.Base(kernelImage))
	// "Image.gz-dtb" is the conventional AnyKernel3 image name; preserve it.
	if strings.HasSuffix(kernelImage, "Image.gz-dtb") {
		dest = filepath.Join(anyKernelDir, "Image.gz-dtb")
	}
	if err := copyFile(kernelImage, dest); err != nil {
		return err
	}
	return zipDir(anyKernelDir, outputZip)
}

// CloneIfMissing ensures `dir` is a usable AnyKernel3 checkout.
// If missing or empty, it clones osm0sis/AnyKernel3.
func CloneIfMissing(ctx context.Context, dir string, line proc.LineFunc) error {
	if dirExists(dir) && fileExists(filepath.Join(dir, "anykernel.sh")) {
		return nil
	}
	if dirExists(dir) {
		// Treat as fresh; clone into a temp dir and move into place.
		_ = os.RemoveAll(dir)
	}
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	r := proc.Run(ctx, parent, nil, line,
		"git", "clone", "--depth=1", "https://github.com/osm0sis/AnyKernel3", dir)
	if !r.Ok() {
		return fmt.Errorf("clone AnyKernel3: exit %d", r.ExitCode)
	}
	return nil
}

func zipDir(srcDir, dstZip string) error {
	out, err := os.Create(dstZip)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip the .git dir entirely.
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(rel, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate
		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// Errors exposed for callers wanting structured comparison.
var (
	ErrNoImage = errors.New("kernel image not found")
)
