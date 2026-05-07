package cmd

import (
	"fmt"

	"github.com/jblwilliams/cosmos-cli/internal/auth"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(logoutCmd)
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear saved credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := auth.Clear(); err != nil {
			return err
		}
		fmt.Println("Logged out.")
		return nil
	},
}
