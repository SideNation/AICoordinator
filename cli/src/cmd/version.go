package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// versionString is the build version. It defaults to "dev" and is overridden
// at release time via -ldflags "-X main.version=<tag>" (see SetVersion).
var versionString = "dev"

// SetVersion records the build version (called from main). Empty values are
// ignored so the "dev" default stays.
func SetVersion(v string) {
	if strings.TrimSpace(v) != "" {
		versionString = v
	}
	rootCmd.Version = versionString
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the aico version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(versionString)
	},
}

func init() {
	rootCmd.Version = versionString
	rootCmd.AddCommand(versionCmd)
}
