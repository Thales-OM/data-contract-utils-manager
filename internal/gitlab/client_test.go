package gitlab

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReleases(t *testing.T) {
	var gotAuth string
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("PRIVATE-TOKEN")
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `[{"tag_name":"v1.4.0","assets":{"links":[{"name":"dcu.exe","url":"/g/p/-/package_files/1/download"}]}}]`)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "group/project", "tok")
	releases, err := client.Releases(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if gotAuth != "tok" {
		t.Errorf("PRIVATE-TOKEN header = %q, want tok", gotAuth)
	}
	if want := "/api/v4/projects/group%2Fproject/releases"; gotPath != want {
		t.Errorf("request path = %q, want %q", gotPath, want)
	}
	if len(releases) != 1 || releases[0].TagName != "v1.4.0" || releases[0].Assets.Links[0].Name != "dcu.exe" {
		t.Errorf("unexpected releases: %+v", releases)
	}
}

func TestReleasesErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "1", "")
	_, err := client.Releases(context.Background())
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v, want status 403 mention", err)
	}
}

func TestDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "binary-content")
	}))
	defer srv.Close()

	var buf strings.Builder
	client := NewClient(srv.URL, "1", "")
	if err := client.Download(context.Background(), srv.URL+"/file", &buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "binary-content" {
		t.Errorf("downloaded %q", buf.String())
	}
}
