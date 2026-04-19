package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexturecorp/aico/src/agent"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agent source files",
}

// shared flags
var (
	flagOutDir  string
	flagDryRun  bool
	flagForce   bool
	flagStrict  bool
	flagPrune   bool
	flagOnly    string
)

func init() {
	agentCmd.AddCommand(splitCmd, buildCmd, validateCmd)

	for _, cmd := range []*cobra.Command{splitCmd, buildCmd} {
		cmd.Flags().StringVar(&flagOutDir, "out-dir", "packages/agents", "output base directory")
		cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "print output to stdout without writing files")
		cmd.Flags().BoolVar(&flagForce, "force", false, "overwrite existing files regardless of mtime")
		cmd.Flags().BoolVar(&flagStrict, "strict", false, "fail on unmappable values instead of warning")
		cmd.Flags().BoolVar(&flagPrune, "prune", false, "delete stale output files excluded by useonly")
		cmd.Flags().StringVar(&flagOnly, "only", "", "override target: claude or opencode")
	}
}

// ---------- split ----------

var splitCmd = &cobra.Command{
	Use:   "split <source.md>",
	Short: "Split a single source agent file into claude and opencode outputs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSplit(args[0])
	},
}

func runSplit(srcPath string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", srcPath, err)
	}
	info, _ := os.Stat(srcPath)
	var srcMtime int64
	if info != nil {
		srcMtime = info.ModTime().Unix()
	}

	src, err := agent.Parse(string(data))
	if err != nil {
		return err
	}

	if errs := agent.Validate(src); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "validation error: %s\n", e)
		}
		return fmt.Errorf("validation failed")
	}

	buildClaude := agent.ShouldBuildClaude(src, flagOnly)
	buildOC := agent.ShouldBuildOpencode(src, flagOnly)

	name := src.Name
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(srcPath), ".md")
	}

	warnStale(name, buildClaude, buildOC, flagOutDir, flagPrune)

	if buildClaude {
		if err := processClaude(src, name, srcMtime); err != nil {
			return err
		}
	}
	if buildOC {
		if err := processOpencode(src, name, srcMtime); err != nil {
			return err
		}
	}
	return nil
}

func processClaude(src *agent.Source, name string, srcMtime int64) error {
	out, warnings, err := agent.TransformClaude(src, flagStrict)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning (claude): %s\n", w)
	}
	rendered, err := agent.RenderClaude(out, src.Body)
	if err != nil {
		return err
	}
	return writeOrPrint("claude", name, rendered, srcMtime)
}

func processOpencode(src *agent.Source, name string, srcMtime int64) error {
	out, warnings, err := agent.TransformOpencode(src, flagStrict)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning (opencode): %s\n", w)
	}
	rendered, err := agent.RenderOpencode(out, src.Body)
	if err != nil {
		return err
	}
	return writeOrPrint("opencode", name, rendered, srcMtime)
}

func writeOrPrint(target, name string, data []byte, srcMtime int64) error {
	if flagDryRun {
		fmt.Printf("=== %s/%s.md ===\n%s\n", target, name, data)
		return nil
	}
	outPath := filepath.Join(flagOutDir, target, name+".md")
	wrote, err := agent.WriteFile(outPath, data, srcMtime, flagForce)
	if err != nil {
		return err
	}
	if wrote {
		fmt.Printf("wrote %s\n", outPath)
	} else {
		fmt.Printf("up-to-date %s\n", outPath)
	}
	return nil
}

func warnStale(name string, buildClaude, buildOC bool, outDir string, prune bool) {
	check := func(target string, shouldBuild bool) {
		if shouldBuild {
			return
		}
		path := filepath.Join(outDir, target, name+".md")
		if _, err := os.Stat(path); err == nil {
			if prune {
				os.Remove(path)
				fmt.Printf("pruned stale %s\n", path)
			} else {
				fmt.Fprintf(os.Stderr, "warning: stale output exists at %s (not built by useonly); use --prune to remove\n", path)
			}
		}
	}
	check("claude", buildClaude)
	check("opencode", buildOC)
}

// ---------- build ----------

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build all agent source files from subagents/ directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		srcDir, _ := cmd.Flags().GetString("src")
		return runBuild(srcDir)
	},
}

func init() {
	buildCmd.Flags().String("src", "subagents", "source directory containing agent .md files")
}

func runBuild(srcDir string) error {
	entries, err := filepath.Glob(filepath.Join(srcDir, "*.md"))
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("no .md files found in %s", srcDir)
	}

	names := map[string]string{}
	var errs []string
	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Sprintf("read %s: %v", path, err))
			continue
		}
		src, err := agent.Parse(string(data))
		if err != nil {
			errs = append(errs, fmt.Sprintf("parse %s: %v", path, err))
			continue
		}
		name := src.Name
		if name == "" {
			name = strings.TrimSuffix(filepath.Base(path), ".md")
		}
		if prev, exists := names[name]; exists {
			errs = append(errs, fmt.Sprintf("duplicate name %q in %s and %s", name, prev, path))
			continue
		}
		names[name] = path
	}
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "error: %s\n", e)
		}
		return fmt.Errorf("build aborted due to errors")
	}

	for _, path := range entries {
		if err := runSplit(path); err != nil {
			fmt.Fprintf(os.Stderr, "error processing %s: %v\n", path, err)
		}
	}
	return nil
}

// ---------- validate ----------

var validateCmd = &cobra.Command{
	Use:   "validate <source.md>",
	Short: "Validate a source agent file without writing output",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		src, err := agent.Parse(string(data))
		if err != nil {
			return err
		}
		errs := agent.Validate(src)
		if len(errs) == 0 {
			fmt.Println("ok")
			return nil
		}
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "error: %s\n", e)
		}
		os.Exit(1)
		return nil
	},
}
