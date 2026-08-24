package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

func TestLoadPrecedence(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), ".env")
	content := "GITLAB_TOKEN=from-file\nGITLAB_BASE_URL=https://file.example.com\n"
	if err := os.WriteFile(envFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvToken, "from-env")
	t.Setenv(EnvBaseURL, "")

	t.Run("env beats file", func(t *testing.T) {
		cfg, err := Load(envFile, "")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Token != "from-env" {
			t.Errorf("token = %q, want from-env", cfg.Token)
		}
		if cfg.BaseURL != "https://file.example.com" {
			t.Errorf("base URL = %q, want file value", cfg.BaseURL)
		}
	})

	t.Run("flag beats everything", func(t *testing.T) {
		cfg, err := Load(envFile, "from-flag")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Token != "from-flag" {
			t.Errorf("token = %q, want from-flag", cfg.Token)
		}
	})
}

func TestLoadTrimsTrailingSlash(t *testing.T) {
	t.Setenv(EnvBaseURL, "https://gitlab.example.com/")

	cfg, err := Load("", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "https://gitlab.example.com" {
		t.Errorf("base URL = %q", cfg.BaseURL)
	}
}

func TestValidate(t *testing.T) {
	full := &Config{BaseURL: "https://x", ProjectID: "1"}
	if err := full.Validate(); err != nil {
		t.Errorf("full config rejected: %v", err)
	}

	empty := &Config{}
	err := empty.Validate()
	if err == nil {
		t.Fatal("empty config accepted")
	}
	for _, key := range []string{EnvBaseURL, EnvProjectID} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error %q does not mention %s", err, key)
		}
	}
}

func TestSaveValueKeepsOtherEntries(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envFile, []byte("EXISTING=keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := SaveValue(envFile, EnvToken, "secret"); err != nil {
		t.Fatal(err)
	}

	values, err := godotenv.Read(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if values["EXISTING"] != "keep" {
		t.Errorf("EXISTING = %q, want keep (other entries must survive)", values["EXISTING"])
	}
	if values[EnvToken] != "secret" {
		t.Errorf("GITLAB_TOKEN = %q, want secret", values[EnvToken])
	}
}
