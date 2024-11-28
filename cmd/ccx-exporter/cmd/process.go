package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// processCmd represents the process command
var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Process kafka events & push it to s3",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("process called")
	},
}

func init() {
	rootCmd.AddCommand(processCmd)
}
