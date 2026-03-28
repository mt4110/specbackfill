package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mt4110/specbackfill/internal/checkcmd"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: specbackfill check [flags]")
		os.Exit(2)
	}

	if os.Args[1] != "check" {
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "usage: specbackfill check [flags]")
		os.Exit(2)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: determine working directory: %v\n", err)
		os.Exit(2)
	}

	code := checkcmd.Run(context.Background(), cwd, os.Args[2:], os.Stdout, os.Stderr)
	os.Exit(code)
}
