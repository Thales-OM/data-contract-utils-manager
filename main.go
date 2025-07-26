package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"net/url"
	"path"
	"regexp"
)

const (
	gitlabReleasesAPI = "https://gitlab.wildberries.ru/api/v4/projects/20968/releases"
	envFileName       = ".env"
	cacheDirName      = "cache"
	currentDirName    = "current"
	metaFileName      = "meta.yaml"
	tokenEnvKey       = "GITLAB_TOKEN"
)

// Extensions considered non-executable (text/config/archive)
var nonExecutableExtensions = []string{
	".yaml", ".yml", ".json", ".txt", ".md", ".zip", ".tar.gz", ".tar.bz2", ".tar", ".gz", ".bz2", ".dll", ".so", ".dylib",
}

// List of typical executable extensions
var executableExtensions = []string{
	".exe", ".bat", ".sh", ".bin", ".cmd", ".app", ".run", // Windows, Unix, and macOS
}

// Release represents a GitLab release JSON object (partial)
type Release struct {
	Name        string    `json:"name"`
	TagName     string    `json:"tag_name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	ReleasedAt  time.Time `json:"released_at"`
	Assets      Assets    `json:"assets"`
}

type Assets struct {
	Count   int      `json:"count"`
	Sources []Source `json:"sources"`
	Links   []Link   `json:"links"`
}

type Source struct {
	Format string `json:"format"`
	URL    string `json:"url"`
}

type Link struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	DirectAssetURL string `json:"direct_asset_url"`
	LinkType       string `json:"link_type"`
}

type Meta struct {
	Versions map[string]string `yaml:"versions"` // version -> cached executable filename
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "data-contract-utils-manager",
		Short: "Manage different versions of data-contract-utils applications from GitLab",
	}

	rootCmd.AddCommand(setTokenCmd())
	rootCmd.AddCommand(switchCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func setTokenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-token <token>",
		Short: "Set and save GitLab personal token to .env file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			token := args[0]
			return saveTokenToEnv(token)
		},
	}
}

func saveTokenToEnv(token string) error {
	envPath, err := getEnvFilePath()
	if err != nil {
		return err
	}

	envMap := map[string]string{}
	// Load existing env if exists
	if _, err := os.Stat(envPath); err == nil {
		envMap, err = godotenv.Read(envPath)
		if err != nil {
			return fmt.Errorf("failed to read existing .env: %w", err)
		}
	}

	envMap[tokenEnvKey] = token

	err = godotenv.Write(envMap, envPath)
	if err != nil {
		return fmt.Errorf("failed to write .env file: %w", err)
	}

	fmt.Printf("Token saved to %s\n", envPath)
	return nil
}

func getEnvFilePath() (string, error) {
	// .env is alongside executable
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	execDir := filepath.Dir(execPath)
	return filepath.Join(execDir, envFileName), nil
}

func inferPackageName(version string) string {
	// Split the version string by "."
	parts := strings.Split(version, ".")
	if len(parts) < 3 {
		return "data-contract-utils" // If version format is invalid, default to "data-contract-utils"
	}
	// Convert the major version to an integer
	majorVersion, err := strconv.Atoi(parts[0])
	if err != nil {
		return "data-contract-utils" // If conversion fails, default to "data-contract-utils"
	}
	// Check if major version is 1 and minor version is 3 or above
	if majorVersion > 1 || (majorVersion == 1 && parts[1] >= "3") {
		return "data-contract-utils"
	}
	return "helper"
}

func fixGitlabPackageFileURL(rawURL string, version string, fileName string) (string, error) {
	// transformGitlabPackageFileURL transforms URLs matching the package_files pattern into API download URLs.
	// If URL does not match pattern, returns it unchanged.

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Use regex to extract project_path from path
	// Pattern: ^/(.+)/-/package_files/\d+/download$
	re := regexp.MustCompile(`^/(.+)/-/package_files/\d+/download$`)
	matches := re.FindStringSubmatch(parsed.Path)
	if len(matches) != 2 {
		// No match, return original URL
		return rawURL, nil
	}

	projectPath := matches[1]

	// Escape project path for API URL
	escapedProjectPath := url.PathEscape(projectPath)

	package_name := inferPackageName(version)

	// Construct new URL:
	// {scheme}://{host}/api/v4/projects/{escapedProjectPath}/packages/generic/{lastSegment}/{version}/{fileName}
	newPath := path.Join(
		"api", "v4", "projects", escapedProjectPath,
		"packages", "generic", package_name, version, fileName,
	)

	// Manually construct the full URL to avoid double-escaping
	newURL := fmt.Sprintf("%s://%s/%s", parsed.Scheme, parsed.Host, newPath)

	return newURL, nil
}

func switchCmd() *cobra.Command {
	var tokenFlag string
	var nameFlag string
	var forceFlag bool

	cmd := &cobra.Command{
		Use:   "switch <version>",
		Short: "Switch to specified version of data-contract-utils",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			version := args[0]

			// Load token from flag or .env
			token, err := getToken(tokenFlag)
			if err != nil {
				return err
			}

			// Ensure directories exist
			if err := ensureDir(cacheDirName); err != nil {
				return err
			}
			if err := ensureDir(currentDirName); err != nil {
				return err
			}

			// Load meta.yaml
			meta, err := loadMeta()
			if err != nil {
				return err
			}

			var cachedExeName string
			var formattedURL string

			// Check if version cached
			// Skip if forced to download
			if !forceFlag {
				cachedExeName, cached := meta.Versions[version]
				if cached && (nameFlag == "" || nameFlag == cachedExeName) {
					fmt.Printf("Version %s found in cache.\n", version)
					return switchToCachedVersion(version, cachedExeName)
				}

				if cached && (nameFlag != "" && nameFlag != cachedExeName) {
					fmt.Printf("Version %s found in cache but does not match the given name. Redownloading...\n", version)
				}
			}

			// Not cached, fetch releases from GitLab
			releases, err := fetchReleases(token)
			if err != nil {
				return err
			}

			// Find release by tag_name
			var release *Release
			for _, r := range releases {
				if r.TagName == version {
					release = &r
					break
				}
			}
			if release == nil {
				return fmt.Errorf("release with version tag %s not found", version)
			}

			// Filter executable links (exclude non-executable extensions)
			execLinks := filterExecutableLinks(release.Assets.Links)

			if len(execLinks) == 0 {
				return errors.New("no executable assets found in release")
			}

			var selectedLink Link
			if nameFlag != "" {
				// Find by name
				found := false
				for _, l := range execLinks {
					if l.Name == nameFlag {
						selectedLink = l
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("executable with name %s not found in release", nameFlag)
				}
			} else {
				// Prompt user to select
				selectedLink, err = promptUserToSelect(execLinks)
				if err != nil {
					return err
				}
			}
			
			formattedURL, err = fixGitlabPackageFileURL(selectedLink.URL, version, selectedLink.Name)
			if err != nil {
				return err
			}

			// Download executable to cache
			cachedExeName, err = downloadToCache(formattedURL, selectedLink.Name, token)
			if err != nil {
				return err
			}

			// Update meta.yaml
			meta.Versions[version] = cachedExeName
			if err := saveMeta(meta); err != nil {
				return err
			}

			// Switch to cached version
			return switchToCachedVersion(version, cachedExeName)
		},
	}

	cmd.Flags().StringVar(&tokenFlag, "token", "", "GitLab personal token (optional, overrides .env)")
	cmd.Flags().StringVar(&nameFlag, "name", "", "Name of executable asset to download (optional)")
	cmd.Flags().BoolVar(&forceFlag, "force", false, "Force redownload version executable")

	return cmd
}

func getToken(flagToken string) (string, error) {
	if flagToken != "" {
		return flagToken, nil
	}

	envPath, err := getEnvFilePath()
	if err != nil {
		return "", err
	}

	envMap, err := godotenv.Read(envPath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to read .env file: %w", err)
	}

	token := envMap[tokenEnvKey]
	return token, nil // token can be empty (allowed)
}

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

func loadMeta() (*Meta, error) {
	metaPath := filepath.Join(cacheDirName, metaFileName)
	meta := &Meta{Versions: map[string]string{}}

	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		return meta, nil // no meta file yet
	}

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read meta file: %w", err)
	}

	if err := yaml.Unmarshal(data, meta); err != nil {
		return nil, fmt.Errorf("failed to parse meta file: %w", err)
	}

	if meta.Versions == nil {
		meta.Versions = map[string]string{}
	}

	return meta, nil
}

func saveMeta(meta *Meta) error {
	metaPath := filepath.Join(cacheDirName, metaFileName)
	data, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal meta data: %w", err)
	}

	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write meta file: %w", err)
	}

	return nil
}

func fetchReleases(token string) ([]Release, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", gitlabReleasesAPI, nil)
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch releases: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var releases []Release
	decoder := jsonNewDecoder(resp.Body)
	if err := decoder.Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode releases JSON: %w", err)
	}

	return releases, nil
}

func filterExecutableLinks(links []Link) []Link {
	var filtered []Link
	for _, l := range links {
		if isExecutableFile(l.Name) {
			filtered = append(filtered, l)
		}
	}
	return filtered
}

func isExecutableFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, neExt := range nonExecutableExtensions {
		if ext == neExt {
			return false
		}
	}
	return true
}

func promptUserToSelect(links []Link) (Link, error) {
	fmt.Println("Select executable to download:")
	for i, l := range links {
		fmt.Printf("%d) %s\n", i+1, l.Name)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Enter number (1-%d): ", len(links))
		input, err := reader.ReadString('\n')
		if err != nil {
			return Link{}, err
		}
		input = strings.TrimSpace(input)
		num, err := strconv.Atoi(input)
		if err != nil || num < 1 || num > len(links) {
			fmt.Println("Invalid input, try again.")
			continue
		}
		return links[num-1], nil
	}
}

func downloadToCache(downloadURL string, name string, token string) (string, error) {
	fmt.Printf("Downloading %s ...\n", name)

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return "", err
	}
	
	if token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to download file: status %d", resp.StatusCode)
	}

	cachedFilePath := filepath.Join(cacheDirName, name)

	outFile, err := os.Create(cachedFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create cache file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save downloaded file: %w", err)
	}

	// Make executable if on Unix
	if runtime.GOOS != "windows" {
		if err := os.Chmod(cachedFilePath, 0755); err != nil {
			return "", fmt.Errorf("failed to chmod downloaded file: %w", err)
		}
	}

	fmt.Printf("Downloaded and cached as %s\n", name)
	return name, nil
}

func switchToCachedVersion(version, cachedExeName string) error {
	if err := cleanDir(currentDirName); err != nil {
		return err
	}

	srcPath := filepath.Join(cacheDirName, cachedExeName)
	simplifiedName := simplifiedExecutableName(cachedExeName)
	dstPath := filepath.Join(currentDirName, simplifiedName)

	if err := copyFile(srcPath, dstPath); err != nil {
		return fmt.Errorf("failed to copy executable to current: %w", err)
	}

	// Make executable if on Unix
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dstPath, 0755); err != nil {
			return fmt.Errorf("failed to chmod current executable: %w", err)
		}
	}

	fmt.Printf("Switched to version %s. Executable placed at %s\n", version, dstPath)
	return nil
}

func cleanDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		err := os.RemoveAll(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}

func getExecutableExtension(filename string) string {
	// Check each extension in the list
	for _, ext := range executableExtensions {
		if strings.HasSuffix(filename, ext) {
			return ext
		}
	}
	return "" // No valid extension found
}

func simplifiedExecutableName(name string) string {
	idx := strings.Index(name, "-")
	if idx == -1 {
		return name
	}
	short_name := name[:idx]
	extension := getExecutableExtension(name)
	// extension will be "" if none found
	return short_name + extension
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return out.Sync()
}

// jsonNewDecoder is a helper to avoid import cycle with encoding/json
func jsonNewDecoder(r io.Reader) *jsonDecoder {
	return &jsonDecoder{r: r}
}

type jsonDecoder struct {
	r io.Reader
}

func (d *jsonDecoder) Decode(v interface{}) error {
	return jsonDecode(d.r, v)
}

func jsonDecode(r io.Reader, v interface{}) error {
	return json.NewDecoder(r).Decode(v)
}
