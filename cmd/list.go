package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/Thales-OM/data-contract-utils-manager/internal/config"
	"github.com/Thales-OM/data-contract-utils-manager/internal/gitlab"
	"github.com/Thales-OM/data-contract-utils-manager/internal/store"
)

func newListCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available releases, cached versions and the active one",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.OutOrStdout(), cmd.ErrOrStderr(), opts)
		},
	}
}

func runList(out, errOut io.Writer, opts *options) error {
	envPath, err := config.DefaultEnvPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(envPath, opts.token)
	if err != nil {
		return err
	}

	rootDir, err := workingRoot()
	if err != nil {
		return err
	}
	st := store.New(rootDir)

	meta, err := st.LoadMeta()
	if err != nil {
		return err
	}

	// The remote part is best-effort: a broken configuration must not hide
	// the locally available information.
	if validateErr := cfg.Validate(); validateErr != nil {
		fmt.Fprintln(errOut, "Remote listing unavailable:", validateErr)
	} else {
		client := gitlab.NewClient(cfg.BaseURL, cfg.ProjectID, cfg.Token)
		if fetchErr := printReleases(out, client, cfg.ProjectID, meta); fetchErr != nil {
			fmt.Fprintln(errOut, "Failed to fetch releases:", fetchErr)
		}
	}

	cached, err := st.CachedVersions()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "\nCached assets (%s):\n", st.CacheDir())
	if len(cached) == 0 {
		fmt.Fprintln(out, "  (cache is empty)")
	}
	for _, name := range cached {
		fmt.Fprintf(out, "  %s\n", name)
	}

	if meta.Current != "" {
		fmt.Fprintf(out, "\nActive version: %s\n", meta.Current)
	} else {
		fmt.Fprintln(out, "\nActive version: none (run `switch <version>` first)")
	}
	return nil
}

func printReleases(out io.Writer, client *gitlab.Client, projectID string, meta *store.Meta) error {
	releases, err := client.Releases(context.Background())
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Releases of project %s:\n", projectID)
	for _, release := range releases {
		marker := " "
		switch {
		case release.TagName == meta.Current:
			marker = "*"
		case isCached(meta, release.TagName):
			marker = "+"
		}
		fmt.Fprintf(out, "%s %-12s %-40s %s\n",
			marker,
			release.TagName,
			truncate(release.Name, 40),
			formatReleaseDate(release),
		)
	}
	fmt.Fprint(out, "\n(* active, + cached)\n")
	return nil
}

func isCached(meta *store.Meta, tagName string) bool {
	_, ok := meta.Versions[tagName]
	return ok
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func formatReleaseDate(release gitlab.Release) string {
	date := release.ReleasedAt
	if date.IsZero() {
		date = release.CreatedAt
	}
	if date.IsZero() {
		return "-"
	}
	return date.Format("2006-01-02")
}
