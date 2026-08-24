// Package gitlab provides a minimal client for the parts of the GitLab REST
// API required to discover project releases and download their assets.
package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	apiPrefix       = "/api/v4"
	releasesTimeout = 30 * time.Second
	downloadTimeout = 15 * time.Minute
	errorSnippetMax = 512
)

// Client talks to the releases of a single GitLab project.
type Client struct {
	// BaseURL is the GitLab instance root, e.g. "https://gitlab.example.com".
	BaseURL string

	// ProjectID is a numeric project ID or URL-encoded project path.
	ProjectID string

	// Token is an optional personal access token sent as PRIVATE-TOKEN.
	Token string

	// HTTP is the underlying HTTP client; a default is used when nil.
	HTTP *http.Client
}

// NewClient builds a client for the given instance and project.
func NewClient(baseURL, projectID, token string) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		ProjectID: url.PathEscape(projectID),
		Token:     token,
	}
}

// Release is a partial view of a GitLab release object.
type Release struct {
	Name        string    `json:"name"`
	TagName     string    `json:"tag_name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	ReleasedAt  time.Time `json:"released_at"`
	Assets      Assets    `json:"assets"`
}

// Assets groups downloadable artifacts attached to a release.
type Assets struct {
	Count   int      `json:"count"`
	Sources []Source `json:"sources"`
	Links   []Link   `json:"links"`
}

// Source is an auto-generated source code archive.
type Source struct {
	Format string `json:"format"`
	URL    string `json:"url"`
}

// Link points at a single asset attached to a release.
type Link struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	DirectAssetURL string `json:"direct_asset_url"`
	LinkType       string `json:"link_type"`
}

// Releases lists all releases of the project, newest first.
func (c *Client) Releases(ctx context.Context) ([]Release, error) {
	ctx, cancel := context.WithTimeout(ctx, releasesTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("%s%s/projects/%s/releases", c.BaseURL, apiPrefix, c.ProjectID)
	body, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()

	var releases []Release
	if err := json.NewDecoder(body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode releases response: %w", err)
	}
	return releases, nil
}

// Download streams the file behind assetURL into dst.
func (c *Client) Download(ctx context.Context, assetURL string, dst io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	body, err := c.get(ctx, assetURL)
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()

	if _, err := io.Copy(dst, body); err != nil {
		return fmt.Errorf("download asset: %w", err)
	}
	return nil
}

func (c *Client) get(ctx context.Context, endpoint string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", endpoint, err)
	}
	if c.Token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.Token)
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", endpoint, err)
	}
	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, errorSnippetMax))
		return nil, fmt.Errorf("GET %s: unexpected status %d, body: %s",
			endpoint, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return resp.Body, nil
}
