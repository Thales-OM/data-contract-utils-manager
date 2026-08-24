package gitlab

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// nonExecutableExtensions lists asset file extensions that can never be the
// runnable artifact a user wants to install (docs, archives, libraries, ...).
var nonExecutableExtensions = map[string]bool{
	".yaml": true, ".yml": true, ".json": true, ".txt": true, ".md": true,
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true,
	".dll": true, ".so": true, ".dylib": true,
	".deb": true, ".rpm": true, ".pkg": true,
}

// packageFilePattern matches GitLab web URLs of release package files:
//
//	/<group>/<project>/-/package_files/<id>/download
var packageFilePattern = regexp.MustCompile(`^/(.+)/-/package_files/\d+/download$`)

const (
	currentPackage = "data-contract-utils"
	legacyPackage  = "helper"
)

// IsExecutableAsset reports whether an asset looks like a runnable binary
// rather than documentation, an archive or a library.
func IsExecutableAsset(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return !nonExecutableExtensions[ext]
}

// ExecutableAssets returns the subset of links that look like binaries.
func ExecutableAssets(links []Link) []Link {
	filtered := make([]Link, 0, len(links))
	for _, link := range links {
		if IsExecutableAsset(link.Name) {
			filtered = append(filtered, link)
		}
	}
	return filtered
}

// FindAsset returns the link with exactly the given display name.
func FindAsset(links []Link, name string) (Link, bool) {
	for _, link := range links {
		if link.Name == name {
			return link, true
		}
	}
	return Link{}, false
}

// InferPackageName maps a semantic version to the generic package that stores
// the built assets. Versions before 1.3.0 were published under a legacy name.
func InferPackageName(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 3 {
		return currentPackage
	}
	major, err := strconv.Atoi(parts[0])
	minor, minorErr := strconv.Atoi(parts[1])
	if err != nil || minorErr != nil {
		return currentPackage
	}
	if major > 1 || (major == 1 && minor >= 3) {
		return currentPackage
	}
	return legacyPackage
}

// AssetDownloadURL turns the web URL of a release asset into the direct GitLab
// API download URL of the underlying generic package file — the endpoint that
// accepts PRIVATE-TOKEN authentication. URLs outside the package_files pattern
// are returned unchanged.
func AssetDownloadURL(rawURL, version, fileName string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse asset URL %q: %w", rawURL, err)
	}

	matches := packageFilePattern.FindStringSubmatch(parsed.Path)
	if matches == nil {
		return rawURL, nil
	}

	projectPath := url.PathEscape(matches[1])
	apiPath := path.Join(
		"api", "v4", "projects", projectPath,
		"packages", "generic", InferPackageName(version), version, fileName,
	)
	return fmt.Sprintf("%s://%s/%s", parsed.Scheme, parsed.Host, apiPath), nil
}
