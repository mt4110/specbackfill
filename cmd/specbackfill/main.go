package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/mt4110/specbackfill/internal/checkcmd"
	"github.com/mt4110/specbackfill/internal/fixturecmd"
	"github.com/mt4110/specbackfill/internal/rulescmd"
)

const defaultVersion = "v0"

var (
	version = defaultVersion
	commit  = "unknown"
	built   = "unknown"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		writeUsage(stderr)
		return 2
	}

	switch args[0] {
	case "--version":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "error: --version does not accept arguments")
			return 2
		}
		writeVersion(stdout)
		return 0
	case "check":
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "error: determine working directory: %v\n", err)
			return 2
		}
		return checkcmd.RunWithOptions(ctx, cwd, args[1:], stdout, stderr, checkcmd.Options{
			ToolVersion: effectiveVersion(),
		})
	case "rules":
		return rulescmd.Run(args[1:], stdout, stderr)
	case "fixtures":
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "error: determine working directory: %v\n", err)
			return 2
		}
		return fixturecmd.Run(ctx, cwd, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		writeUsage(stderr)
		return 2
	}
}

func writeVersion(w io.Writer) {
	fmt.Fprintf(w, "specbackfill %s commit=%s built=%s\n", effectiveVersion(), cleanBuildValue(commit), cleanBuildValue(built))
}

func effectiveVersion() string {
	cleaned := strings.TrimSpace(version)
	if cleaned != "" && cleaned != defaultVersion {
		return cleaned
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		moduleVersion := strings.TrimSpace(info.Main.Version)
		if moduleVersion != "" && moduleVersion != "(devel)" {
			return moduleVersion
		}
	}

	if cleaned != "" {
		return cleaned
	}
	return defaultVersion
}

func cleanBuildValue(value string) string {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return "unknown"
	}
	return cleaned
}

func writeUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: specbackfill --version")
	fmt.Fprintln(w, "       specbackfill check [flags]")
	fmt.Fprintln(w, "       specbackfill rules list")
	fmt.Fprintln(w, "       specbackfill rules show <RULE_ID>")
	fmt.Fprintln(w, "       specbackfill fixtures report")
}
