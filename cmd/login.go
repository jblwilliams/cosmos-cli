package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/jblwilliams/cosmos-cli/internal/auth"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func init() {
	loginCmd.Flags().StringP("email", "e", "", "cosmos.so email")
	loginCmd.Flags().StringP("password", "p", "", "cosmos.so password (prompted if omitted)")
	loginCmd.Flags().StringP("token", "t", "", "save a bearer token directly")
	rootCmd.AddCommand(loginCmd)
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Save credentials for private collections",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, _ := cmd.Flags().GetString("token")
		if token != "" {
			if err := auth.Save(auth.Data{Token: token}); err != nil {
				return err
			}
			fmt.Println("Token saved.")
			return nil
		}

		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")

		if email == "" {
			fmt.Print("Email: ")
			fmt.Scanln(&email)
		}
		if password == "" {
			fmt.Print("Password: ")
			b, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return fmt.Errorf("reading password: %w", err)
			}
			fmt.Println()
			password = string(b)
		}

		data, err := auth.Login(email, password)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
			os.Exit(1)
		}

		if err := auth.Save(data); err != nil {
			return err
		}
		fmt.Println("Login successful! Credentials saved.")
		return nil
	},
}
