package diffsrc

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func WorkingTree(ctx context.Context, dir string) ([]byte, error) {
	return gitDiff(ctx, dir)
}

func GitRange(ctx context.Context, dir, base, head string) ([]byte, error) {
	return gitDiff(ctx, dir, base, head)
}

func DiffFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read diff file: %w", err)
	}
	return data, nil
}

func gitDiff(ctx context.Context, dir string, args ...string) ([]byte, error) {
	gitArgs := []string{"diff", "--no-ext-diff", "--no-color"}
	gitArgs = append(gitArgs, args...)

	cmd := exec.CommandContext(ctx, "git", gitArgs...)
	cmd.Dir = dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return stdout.Bytes(), nil
}
