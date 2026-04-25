package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildNumberRoundtrip(t *testing.T) {
	dir := t.TempDir()
	// Missing file -> next build is 1
	if got := ReadBuildNumber(dir); got != 1 {
		t.Fatalf("ReadBuildNumber on empty dir = %d, want 1", got)
	}
	// Commit 7 -> next build is 8
	if err := CommitBuildNumber(dir, 7); err != nil {
		t.Fatal(err)
	}
	if got := ReadBuildNumber(dir); got != 8 {
		t.Fatalf("ReadBuildNumber after CommitBuildNumber(7) = %d, want 8", got)
	}
	// Reset zeroes the on-disk file -> next build is 1
	out := filepath.Join(dir, "out")
	if err := ResetBuildNumber(dir, out); err != nil {
		t.Fatal(err)
	}
	if got := ReadBuildNumber(dir); got != 1 {
		t.Fatalf("ReadBuildNumber after ResetBuildNumber = %d, want 1", got)
	}
	if b, err := os.ReadFile(filepath.Join(out, ".version")); err != nil || string(b) != "0\n" {
		t.Fatalf(".version after reset = %q (err=%v), want %q", string(b), err, "0\n")
	}
}

func TestBuilderRoundtrip(t *testing.T) {
	dir := t.TempDir()
	// Default load
	b := LoadBuilder(dir)
	if !b.UseCcache || b.Incremental || b.KSUBranch != "main" || b.ForceCleanReason != "" {
		t.Fatalf("default builder = %+v", b)
	}
	// Save + reload
	b.Incremental = true
	b.UseCcache = false
	b.KSUBranch = "dev"
	b.ForceCleanReason = "Branch switched"
	if err := b.Save(dir); err != nil {
		t.Fatal(err)
	}
	got := LoadBuilder(dir)
	if got != b {
		t.Fatalf("after save/load:\n got %+v\nwant %+v", got, b)
	}
	// Defensive: bogus branch falls back to main on load
	garbage := "INCREMENTAL=true\nKSU_BRANCH=\"weirdbranch\"\n"
	if err := os.WriteFile(filepath.Join(dir, FileBuilderState), []byte(garbage), 0o644); err != nil {
		t.Fatal(err)
	}
	got = LoadBuilder(dir)
	if got.KSUBranch != "main" {
		t.Fatalf("garbage branch should fall back to main, got %q", got.KSUBranch)
	}
}

func TestPrevBuildRoundtrip(t *testing.T) {
	dir := t.TempDir()
	if pb := LoadPrevBuild(dir); !pb.Empty() {
		t.Fatalf("missing file should yield empty PrevBuild, got %+v", pb)
	}
	pb := PrevBuild{
		Cap:        "[DEV-ReSukiSU | Hook-Mode=Manual-Hook]",
		ExtFeat:    "[KPM]",
		Mode:       "Incremental",
		Num:        42,
		KernelName: "AnyMore-v2.1",
		Date:       "2025-04-25 03:14",
	}
	if err := pb.Save(dir); err != nil {
		t.Fatal(err)
	}
	got := LoadPrevBuild(dir)
	if got != pb {
		t.Fatalf("after save/load:\n got %+v\nwant %+v", got, pb)
	}
}

func TestMenuconfigPreserve(t *testing.T) {
	dir := t.TempDir()
	if MenuconfigPreserved(dir) {
		t.Fatal("preserve should be false on empty dir")
	}
	src := filepath.Join(dir, ".config")
	if err := os.WriteFile(src, []byte("CONFIG_FOO=y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveMenuconfigPreserve(dir, src); err != nil {
		t.Fatal(err)
	}
	if !MenuconfigPreserved(dir) {
		t.Fatal("preserve should be true after save")
	}
	b, err := os.ReadFile(MenuconfigPreservePath(dir))
	if err != nil || string(b) != "CONFIG_FOO=y\n" {
		t.Fatalf("preserved content = %q (err=%v)", string(b), err)
	}
	ClearMenuconfigPreserve(dir)
	if MenuconfigPreserved(dir) {
		t.Fatal("preserve should be false after clear")
	}
}
