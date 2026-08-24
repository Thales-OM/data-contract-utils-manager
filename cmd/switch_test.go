package cmd

import (
	"strings"
	"testing"

	"github.com/Thales-OM/data-contract-utils-manager/internal/gitlab"
)

func TestSelectAssetPicksValidNumber(t *testing.T) {
	links := []gitlab.Link{
		{Name: "dcu-windows.exe"},
		{Name: "dcu-linux"},
		{Name: "dcu-macos"},
	}

	var out strings.Builder
	link, err := selectAsset(links, strings.NewReader("nope\n9\n2\n"), &out)
	if err != nil {
		t.Fatal(err)
	}
	if link.Name != "dcu-linux" {
		t.Errorf("selected %q, want dcu-linux", link.Name)
	}
	if !strings.Contains(out.String(), "Select asset [1-3]:") {
		t.Errorf("prompt missing from output:\n%s", out.String())
	}
}

func TestSelectAssetFailsOnEOF(t *testing.T) {
	var out strings.Builder
	if _, err := selectAsset([]gitlab.Link{{Name: "a"}}, strings.NewReader(""), &out); err == nil {
		t.Fatal("expected error on empty input")
	}
}
