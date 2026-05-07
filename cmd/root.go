package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cosmos",
	Short: "Download images from cosmos.so collections",
}

func Execute() error {
	return rootCmd.Execute()
}
