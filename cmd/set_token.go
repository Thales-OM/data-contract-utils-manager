package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Thales-OM/data-contract-utils-manager/internal/config"
)

func newSetTokenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-token <token>",
		Short: "Persist a GitLab personal access token in the .env file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			envPath, err := config.DefaultEnvPath()
			if err != nil {
				return err
			}
			if err := config.SaveValue(envPath, config.EnvToken, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Token saved to %s\n", envPath)
			return nil
		},
	}
}
