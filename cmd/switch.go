package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Thales-OM/data-contract-utils-manager/internal/gitlab"
	"github.com/Thales-OM/data-contract-utils-manager/internal/store"
)

func newSwitchCmd(opts *options) *cobra.Command {
	var assetName string
	var force bool

	cmd := &cobra.Command{
		Use:   "switch <version>",
		Short: "Download (or reuse from cache) a release and make it active",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSwitch(cmd, opts, args[0], assetName, force)
		},
	}

	cmd.Flags().StringVar(&assetName, "name", "",
		"Name of the release asset to download (skips the interactive picker)")
	cmd.Flags().BoolVar(&force, "force", false,
		"Re-download the asset even if the version is already cached")

	return cmd
}

func runSwitch(cmd *cobra.Command, opts *options, version, assetName string, force bool) error {
	a, err := opts.resolve()
	if err != nil {
		return err
	}
	if err := a.store.EnsureDirs(); err != nil {
		return err
	}

	meta, err := a.store.LoadMeta()
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	if !force {
		cachedAsset, cached := meta.Versions[version]
		switch {
		case cached && (assetName == "" || assetName == cachedAsset):
			return activateCached(a, meta, version, cachedAsset)
		case cached:
			fmt.Fprintf(cmd.OutOrStdout(),
				"Version %s is cached as %q which does not match the requested asset %q; re-downloading\n",
				version, cachedAsset, assetName)
		}
	}

	release, err := findRelease(ctx, a.client, version)
	if err != nil {
		return err
	}

	assets := gitlab.ExecutableAssets(release.Assets.Links)
	if len(assets) == 0 {
		return fmt.Errorf("release %s contains no executable assets", version)
	}

	link := assets[0]
	if assetName != "" {
		found, ok := gitlab.FindAsset(assets, assetName)
		if !ok {
			return fmt.Errorf("asset %q not found in release %s; available: %s",
				assetName, version, joinNames(assets))
		}
		link = found
	} else if len(assets) > 1 {
		link, err = selectAsset(assets, cmd.InOrStdin(), cmd.OutOrStdout())
		if err != nil {
			return err
		}
	}

	downloadURL, err := gitlab.AssetDownloadURL(link.URL, version, link.Name)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Downloading %s ...\n", link.Name)
	sink, err := a.store.OpenCacheFile(link.Name)
	if err != nil {
		return err
	}

	if err := a.client.Download(ctx, downloadURL, sink); err != nil {
		_ = sink.Close()
		return err
	}
	if err := sink.Close(); err != nil {
		return fmt.Errorf("finalize %s: %w", link.Name, err)
	}

	meta.Versions[version] = link.Name
	meta.Current = version
	if err := a.store.SaveMeta(meta); err != nil {
		return err
	}

	return activate(a, version, link.Name)
}

// activateCached marks an already-downloaded version active without touching
// the network.
func activateCached(a *app, meta *store.Meta, version, cachedAsset string) error {
	fmt.Printf("Version %s found in cache.\n", version)

	meta.Current = version
	if err := a.store.SaveMeta(meta); err != nil {
		return err
	}
	return activate(a, version, cachedAsset)
}

func activate(a *app, version, cachedAsset string) error {
	path, err := a.store.Activate(cachedAsset)
	if err != nil {
		return err
	}
	fmt.Printf("Switched to version %s. Executable placed at %s\n", version, path)
	return nil
}

func findRelease(ctx context.Context, client *gitlab.Client, version string) (*gitlab.Release, error) {
	releases, err := client.Releases(ctx)
	if err != nil {
		return nil, err
	}
	for i := range releases {
		if releases[i].TagName == version {
			return &releases[i], nil
		}
	}
	return nil, fmt.Errorf("release with tag %q not found", version)
}

func selectAsset(links []gitlab.Link, in io.Reader, out io.Writer) (gitlab.Link, error) {
	fmt.Fprintln(out, "Several executable assets are available:")
	for i, link := range links {
		fmt.Fprintf(out, "  %d) %s\n", i+1, link.Name)
	}

	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprintf(out, "Select asset [1-%d]: ", len(links))
		if !scanner.Scan() {
			return gitlab.Link{}, errors.New("no selection made")
		}

		num, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err == nil && num >= 1 && num <= len(links) {
			return links[num-1], nil
		}
		fmt.Fprintln(out, "Invalid input, try again.")
	}
}

func joinNames(links []gitlab.Link) string {
	names := make([]string, 0, len(links))
	for _, link := range links {
		names = append(names, link.Name)
	}
	return strings.Join(names, ", ")
}
