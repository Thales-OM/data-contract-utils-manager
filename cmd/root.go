// Package cmd wires the command-line interface of data-contract-utils-manager.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Thales-OM/data-contract-utils-manager/internal/config"
	"github.com/Thales-OM/data-contract-utils-manager/internal/gitlab"
	"github.com/Thales-OM/data-contract-utils-manager/internal/store"
)

// Build information, overridable via -ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// options holds flags shared between subcommands.
type options struct {
	token string
}

// app bundles the fully resolved collaborators a command needs.
type app struct {
	cfg    *config.Config
	store  *store.Store
	client *gitlab.Client
}

// Execute runs the root command and terminates the process with a non-zero
// exit code on failure.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	opts := &options{}

	root := &cobra.Command{
		Use:     "data-contract-utils-manager",
		Aliases: []string{"dcum"},
		Short:   "Version manager for the data-contract-utils tool family",
		Long: `data-contract-utils-manager downloads, caches and switches between
versions of the data-contract-utils tools published as GitLab releases.

Downloaded binaries are kept in cache/ next to this executable; the active
version is copied to current/ under its simplified name.

Configuration is resolved from (first match wins):
  1. command-line flags,
  2. environment variables (` + config.EnvBaseURL + `, ` + config.EnvProjectID + `, ` + config.EnvToken + `),
  3. a .env file located next to the executable.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&opts.token, "token", "",
		"GitLab personal access token (overrides "+config.EnvToken+")")

	root.AddCommand(
		newSetTokenCmd(),
		newSwitchCmd(opts),
		newListCmd(opts),
		newVersionCmd(),
	)
	return root
}

// resolve builds the application collaborators from flags, environment and
// the .env file. It fails when mandatory GitLab settings are missing.
func (o *options) resolve() (*app, error) {
	envPath, err := config.DefaultEnvPath()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(envPath, o.token)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	rootDir, err := workingRoot()
	if err != nil {
		return nil, err
	}

	return &app{
		cfg:    cfg,
		store:  store.New(rootDir),
		client: gitlab.NewClient(cfg.BaseURL, cfg.ProjectID, cfg.Token),
	}, nil
}

// workingRoot returns the directory containing the running executable; all
// state lives alongside it so the tool stays self-contained.
func workingRoot() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}
	return filepath.Dir(execPath), nil
}
