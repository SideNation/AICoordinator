package cmd

import (
	"github.com/nexturecorp/aico/src/agent"
	"github.com/nexturecorp/aico/src/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aico",
	Short: "AI agent file manager for Claude Code and opencode",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		rc, err := config.LoadRc()
		if err != nil || rc == nil {
			return nil
		}
		if rc.Models != nil {
			agent.SetModelOverrides(rc.Models)
		}
		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(pluginCmd)
}
