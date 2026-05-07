package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/mt4110/specbackfill/internal/checkcmd"
	"github.com/mt4110/specbackfill/internal/fixturecmd"
	"github.com/mt4110/specbackfill/internal/rulescmd"
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
	case "check":
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "error: determine working directory: %v\n", err)
			return 2
		}
		return checkcmd.Run(ctx, cwd, args[1:], stdout, stderr)
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

func writeUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: specbackfill check [flags]")
	fmt.Fprintln(w, "       specbackfill rules list")
	fmt.Fprintln(w, "       specbackfill rules show <RULE_ID>")
	fmt.Fprintln(w, "       specbackfill fixtures report")
}
