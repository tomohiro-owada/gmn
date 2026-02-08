package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of g",
	Long:  `All software has versions. This is g's.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("g v%s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
