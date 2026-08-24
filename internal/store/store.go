// Package store manages the on-disk state of the tool: the version cache and
// the directory holding the currently activated executable.
//
// Layout below Root:
//
//	cache/meta.yaml   version bookkeeping
//	cache/<asset>     downloaded release executables
//	current/<tool>    simplified copy of the active version
package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	CacheDirName   = "cache"
	CurrentDirName = "current"
	MetaFileName   = "meta.yaml"

	dirPerm  = 0o755
	filePerm = 0o644
	exePerm  = 0o755
)

// Meta is the persisted cache index.
type Meta struct {
	// Current is the release tag activated last; empty when none was yet.
	Current string `yaml:"current,omitempty"`

	// Versions maps a release tag to the cached asset backing it.
	Versions map[string]string `yaml:"versions"`
}

func newMeta() *Meta {
	return &Meta{Versions: map[string]string{}}
}

// Store roots all tool state under a single directory (by default the one
// containing the executable).
type Store struct {
	Root string
}

// New creates a store rooted at dir.
func New(dir string) *Store {
	return &Store{Root: dir}
}

// CacheDir returns the directory holding downloaded assets and meta.yaml.
func (s *Store) CacheDir() string { return filepath.Join(s.Root, CacheDirName) }

// CurrentDir returns the directory holding the active executable.
func (s *Store) CurrentDir() string { return filepath.Join(s.Root, CurrentDirName) }

// MetaPath returns the path of the cache index file.
func (s *Store) MetaPath() string { return filepath.Join(s.CacheDir(), MetaFileName) }

// EnsureDirs creates the cache and current directories if needed.
func (s *Store) EnsureDirs() error {
	for _, dir := range []string{s.CacheDir(), s.CurrentDir()} {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}

// LoadMeta reads the cache index; a missing file yields an empty index.
func (s *Store) LoadMeta() (*Meta, error) {
	data, err := os.ReadFile(s.MetaPath())
	if os.IsNotExist(err) {
		return newMeta(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", s.MetaPath(), err)
	}

	meta := newMeta()
	if err := yaml.Unmarshal(data, meta); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.MetaPath(), err)
	}
	if meta.Versions == nil {
		meta.Versions = map[string]string{}
	}
	return meta, nil
}

// SaveMeta atomically persists the cache index.
func (s *Store) SaveMeta(meta *Meta) error {
	data, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("encode meta: %w", err)
	}

	tmp := s.MetaPath() + ".tmp"
	if err := os.WriteFile(tmp, data, filePerm); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.MetaPath()); err != nil {
		return fmt.Errorf("replace %s: %w", s.MetaPath(), err)
	}
	return nil
}

// CachedVersions lists asset files present in the cache.
func (s *Store) CachedVersions() ([]string, error) {
	entries, err := os.ReadDir(s.CacheDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", s.CacheDir(), err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == MetaFileName || strings.HasSuffix(entry.Name(), ".tmp") {
			continue
		}
		names = append(names, entry.Name())
	}
	return names, nil
}

// OpenCacheFile creates (or truncates) a cache entry for writing.
func (s *Store) OpenCacheFile(name string) (io.WriteCloser, error) {
	dst := filepath.Join(s.CacheDir(), name)
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, exePerm)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", dst, err)
	}
	return f, nil
}

// Activate copies the cached asset into the current directory under its
// simplified name, replacing whatever was there before, and returns the path
// of the activated executable.
func (s *Store) Activate(cachedAsset string) (string, error) {
	src := filepath.Join(s.CacheDir(), cachedAsset)

	if err := os.RemoveAll(s.CurrentDir()); err != nil {
		return "", fmt.Errorf("clean %s: %w", s.CurrentDir(), err)
	}
	if err := os.MkdirAll(s.CurrentDir(), dirPerm); err != nil {
		return "", fmt.Errorf("create %s: %w", s.CurrentDir(), err)
	}

	dst := filepath.Join(s.CurrentDir(), SimplifiedExecutableName(cachedAsset))
	if err := copyFile(src, dst); err != nil {
		return "", err
	}
	return dst, nil
}

// SimplifiedExecutableName strips version/platform suffixes from an asset
// name, e.g. "dcu-1.4.0-windows-amd64.exe" becomes "dcu.exe".
func SimplifiedExecutableName(name string) string {
	base := name
	if idx := strings.Index(name, "-"); idx != -1 {
		base = name[:idx]
	}
	return base + executableExtension(name)
}

// executableExtension returns the known executable suffix of name, if any.
func executableExtension(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range []string{".exe", ".bat", ".cmd", ".sh", ".bin", ".app", ".run"} {
		if strings.HasSuffix(lower, ext) {
			return ext
		}
	}
	return ""
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, exePerm)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy into %s: %w", dst, err)
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return fmt.Errorf("sync %s: %w", dst, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", dst, err)
	}
	return nil
}
