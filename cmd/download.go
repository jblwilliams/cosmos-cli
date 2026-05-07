package cmd

import (
	"fmt"
	"os"

	"github.com/jblwilliams/cosmos-cli/internal/auth"
	"github.com/jblwilliams/cosmos-cli/internal/cosmos"
	"github.com/spf13/cobra"
)

func init() {
	downloadCmd.Flags().StringP("token", "t", "", "override saved auth with this token")
	downloadCmd.Flags().StringP("output", "o", "", "output directory")
	downloadCmd.Flags().Bool("no-skip", false, "re-download existing files")
	downloadCmd.Flags().Float64("delay", 0.1, "delay between downloads in seconds")
	rootCmd.AddCommand(downloadCmd)
}

var downloadCmd = &cobra.Command{
	Use:   "download <collection-url>",
	Short: "Download a collection",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, _ := cmd.Flags().GetString("token")
		output, _ := cmd.Flags().GetString("output")
		noSkip, _ := cmd.Flags().GetBool("no-skip")
		delay, _ := cmd.Flags().GetFloat64("delay")

		client := auth.NewClient(token)

		if token == "" {
			if d, _ := auth.Load(); d == nil {
				fmt.Println("Not logged in. Downloading as anonymous (public collections only).")
				fmt.Println("Run `cosmos login` to access private collections.")
				fmt.Println()
			}
		}

		err := cosmos.DownloadCollection(client, args[0], cosmos.DownloadOptions{
			OutputDir:    output,
			SkipExisting: !noSkip,
			Delay:        delay,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return nil
	},
}
