package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMetaMissingFile(t *testing.T) {
	s := New(t.TempDir())

	meta, err := s.LoadMeta()
	if err != nil {
		t.Fatal(err)
	}
	if meta.Versions == nil {
		t.Error("Versions map is nil, want empty map")
	}
	if meta.Current != "" {
		t.Errorf("Current = %q, want empty", meta.Current)
	}
}

func TestMetaRoundTrip(t *testing.T) {
	s := New(t.TempDir())
	if err := s.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	want := &Meta{
		Current:  "v1.4.0",
		Versions: map[string]string{"v1.4.0": "dcu.exe", "v1.3.9": "dcu-old.exe"},
	}
	if err := s.SaveMeta(want); err != nil {
		t.Fatal(err)
	}

	got, err := s.LoadMeta()
	if err != nil {
		t.Fatal(err)
	}
	if got.Current != want.Current || len(got.Versions) != 2 || got.Versions["v1.4.0"] != "dcu.exe" {
		t.Errorf("round trip mismatch: %+v", got)
	}
}

func TestCachedVersionsSkipsBookkeepingFiles(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	if err := s.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dcu.exe", MetaFileName, "partial.tmp"} {
		if err := os.WriteFile(filepath.Join(s.CacheDir(), name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.CachedVersions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "dcu.exe" {
		t.Errorf("CachedVersions() = %v, want [dcu.exe]", got)
	}
}

func TestActivateCopiesSimplifiedName(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	if err := s.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	cacheFile := filepath.Join(s.CacheDir(), "dcu-1.4.0-windows-amd64.exe")
	if err := os.WriteFile(cacheFile, []byte("MZ-fake-binary"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst, err := s.Activate("dcu-1.4.0-windows-amd64.exe")
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "MZ-fake-binary" {
		t.Errorf("activated content = %q", data)
	}
	if want := filepath.Join(s.CurrentDir(), "dcu.exe"); dst != want {
		t.Errorf("dst = %q, want %q", dst, want)
	}
}

func TestActivateReplacesPreviousContent(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	if err := s.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	writeCache := func(name string) {
		if err := os.WriteFile(filepath.Join(s.CacheDir(), name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeCache("old-tool-1.0.0.exe")
	if _, err := s.Activate("old-tool-1.0.0.exe"); err != nil {
		t.Fatal(err)
	}
	writeCache("new-tool-2.0.0.exe")
	stale := filepath.Join(s.CurrentDir(), "stale.txt")
	if err := os.WriteFile(stale, []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Activate("new-tool-2.0.0.exe"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale file survived activation: %v", err)
	}
}

func TestSimplifiedExecutableName(t *testing.T) {
	cases := map[string]string{
		"dcu-1.4.0-windows-amd64.exe": "dcu.exe",
		"helper-0.9.2.sh":             "helper.sh",
		"plain":                       "plain",
		"DCU-1.0.EXE":                 "DCU.exe",
		"single-name.bin":             "single.bin",
	}
	for in, want := range cases {
		if got := SimplifiedExecutableName(in); got != want {
			t.Errorf("SimplifiedExecutableName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOpenCacheFileTruncatesExisting(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	if err := s.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	name := "asset.exe"
	f, err := s.OpenCacheFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(strings.Repeat("a", 100))); err != nil {
		t.Fatal(err)
	}
	f.Close()

	f, err = s.OpenCacheFile(name)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	info, err := os.Stat(filepath.Join(s.CacheDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Errorf("file size = %d, want 0 (truncated)", info.Size())
	}
}
