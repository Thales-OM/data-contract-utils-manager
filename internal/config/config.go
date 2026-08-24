// Package config resolves runtime configuration for the tool.
//
// Every setting is looked up in the following order, first match wins:
//
//  1. explicit command-line flag,
//  2. process environment variable,
//  3. a .env file located next to the executable.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

const (
	// EnvToken holds a GitLab personal access token.
	EnvToken = "GITLAB_TOKEN"
	// EnvBaseURL holds the root URL of the GitLab instance.
	EnvBaseURL = "GITLAB_BASE_URL"
	// EnvProjectID holds the numeric ID or URL-encoded path of the project
	// whose releases are managed.
	EnvProjectID = "GITLAB_PROJECT_ID"
)

// Config carries everything needed to talk to a GitLab instance.
type Config struct {
	// Token is an optional personal access token; it may be empty when the
	// target project is public.
	Token string

	// BaseURL is the GitLab root URL without trailing slash,
	// e.g. "https://gitlab.example.com".
	BaseURL string

	// ProjectID is a numeric project ID or a URL-encoded "group/project" path.
	ProjectID string
}

// DefaultEnvPath returns the path of the .env file next to the running
// executable, where persistent settings are kept.
func DefaultEnvPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}
	return filepath.Join(filepath.Dir(execPath), ".env"), nil
}

// Load resolves the configuration. envPath may be empty or point to a
// non-existent file; flagToken may be empty as well.
func Load(envPath, flagToken string) (*Config, error) {
	fileValues := make(map[string]string)
	if envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			loaded, err := godotenv.Read(envPath)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", envPath, err)
			}
			for key, value := range loaded {
				fileValues[key] = value
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat %s: %w", envPath, err)
		}
	}

	pick := func(envKey, flagValue string) string {
		switch {
		case flagValue != "":
			return flagValue
		case os.Getenv(envKey) != "":
			return os.Getenv(envKey)
		default:
			return fileValues[envKey]
		}
	}

	return &Config{
		Token:     pick(EnvToken, flagToken),
		BaseURL:   strings.TrimRight(pick(EnvBaseURL, ""), "/"),
		ProjectID: pick(EnvProjectID, ""),
	}, nil
}

// SaveValue persists a single key/value pair into the given .env file while
// preserving all other entries already present.
func SaveValue(envPath, key, value string) error {
	values := make(map[string]string)
	if _, err := os.Stat(envPath); err == nil {
		loaded, err := godotenv.Read(envPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", envPath, err)
		}
		for k, v := range loaded {
			values[k] = v
		}
	}

	values[key] = value
	if err := godotenv.Write(values, envPath); err != nil {
		return fmt.Errorf("write %s: %w", envPath, err)
	}
	return nil
}

// Validate returns an actionable error when mandatory settings are missing.
func (c *Config) Validate() error {
	var missing []string
	if c.BaseURL == "" {
		missing = append(missing, EnvBaseURL+"=https://gitlab.example.com")
	}
	if c.ProjectID == "" {
		missing = append(missing, EnvProjectID+"=1234")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"missing required GitLab settings; provide them via environment variables or a .env file next to the executable:\n  %s",
		strings.Join(missing, "\n  "),
	)
}
