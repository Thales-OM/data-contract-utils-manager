package gitlab

import (
	"strings"
	"testing"
)

func TestIsExecutableAsset(t *testing.T) {
	cases := map[string]bool{
		"dcu-1.4.0-windows-amd64.exe": true,
		"dcu-linux-amd64":             true,
		"tool.sh":                     true,
		"notes.md":                    false,
		"config.yaml":                 false,
		"bundle.tar.gz":               false,
		"lib.dll":                     false,
	}
	for name, want := range cases {
		if got := IsExecutableAsset(name); got != want {
			t.Errorf("IsExecutableAsset(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestExecutableAssets(t *testing.T) {
	links := []Link{
		{Name: "readme.md"},
		{Name: "dcu.exe"},
		{Name: "archive.zip"},
		{Name: "helper"},
	}

	got := ExecutableAssets(links)
	if len(got) != 2 || got[0].Name != "dcu.exe" || got[1].Name != "helper" {
		t.Errorf("ExecutableAssets() = %+v", got)
	}
	if len(links) != 4 {
		t.Errorf("input slice was modified: %+v", links)
	}
}

func TestFindAsset(t *testing.T) {
	links := []Link{{Name: "a.exe"}, {Name: "b.exe"}}

	if _, ok := FindAsset(links, "b.exe"); !ok {
		t.Error("FindAsset(b.exe) not found")
	}
	if _, ok := FindAsset(links, "missing"); ok {
		t.Error("FindAsset(missing) unexpectedly found")
	}
}

func TestInferPackageName(t *testing.T) {
	cases := map[string]string{
		"1.3.0":  currentPackage,
		"1.10.0": currentPackage, // minor compared numerically
		"2.0.0":  currentPackage,
		"1.2.9":  legacyPackage,
		"v1.0.0": currentPackage, // non-numeric prefix falls back to current
		"x.y.z":  currentPackage,
		"1.3":    currentPackage,
	}
	for version, want := range cases {
		if got := InferPackageName(version); got != want {
			t.Errorf("InferPackageName(%q) = %q, want %q", version, got, want)
		}
	}
}

func TestAssetDownloadURL(t *testing.T) {
	cases := []struct {
		name    string
		rawURL  string
		version string
		file    string
		want    string
	}{
		{
			name:    "package file web URL becomes API URL",
			rawURL:  "https://gitlab.example.com/group/proj/-/package_files/42/download",
			version: "1.4.0",
			file:    "dcu.exe",
			want: "https://gitlab.example.com/api/v4/projects/group%2Fproj/" +
				"packages/generic/data-contract-utils/1.4.0/dcu.exe",
		},
		{
			name:    "legacy version maps to legacy package",
			rawURL:  "https://gitlab.example.com/group/proj/-/package_files/7/download",
			version: "1.2.9",
			file:    "helper.exe",
			want: "https://gitlab.example.com/api/v4/projects/group%2Fproj/" +
				"packages/generic/helper/1.2.9/helper.exe",
		},
		{
			name:    "non package URLs pass through unchanged",
			rawURL:  "https://cdn.example.com/some/file.exe",
			version: "1.4.0",
			file:    "file.exe",
			want:    "https://cdn.example.com/some/file.exe",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := AssetDownloadURL(tc.rawURL, tc.version, tc.file)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAssetDownloadURLErrorsOnGarbage(t *testing.T) {
	_, err := AssetDownloadURL("https://example.com/%zz", "1.0.0", "f")
	if err == nil || !strings.Contains(err.Error(), "parse asset URL") {
		t.Fatalf("err = %v, want parse error", err)
	}
}
