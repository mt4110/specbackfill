package diffsrc

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func WorkingTree(ctx context.Context, dir string) ([]byte, error) {
	if hasHead(ctx, dir) {
		return gitDiff(ctx, dir, "HEAD")
	}
	return unbornWorkingTreeDiff(ctx, dir)
}

func GitRange(ctx context.Context, dir, base, head string) ([]byte, error) {
	return gitDiff(ctx, dir, base, head)
}

func DiffFile(dir, path string) ([]byte, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read diff file: %w", err)
	}
	return data, nil
}

func RepoRoot(ctx context.Context, dir string) (string, error) {
	output, err := gitOutput(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("resolve repo root: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func hasHead(ctx context.Context, dir string) bool {
	_, err := gitOutput(ctx, dir, "rev-parse", "--verify", "HEAD")
	return err == nil
}

func unbornWorkingTreeDiff(ctx context.Context, dir string) ([]byte, error) {
	cached, err := gitDiff(ctx, dir, "--cached")
	if err != nil {
		return nil, err
	}

	unstaged, err := gitDiff(ctx, dir)
	if err != nil {
		return nil, err
	}

	if len(cached) == 0 {
		return unstaged, nil
	}
	if len(unstaged) == 0 {
		return cached, nil
	}
	return append(cached, unstaged...), nil
}

func gitDiff(ctx context.Context, dir string, args ...string) ([]byte, error) {
	gitArgs := []string{"diff", "--no-ext-diff", "--no-color"}
	gitArgs = append(gitArgs, args...)

	return gitOutput(ctx, dir, gitArgs...)
}

func gitOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}

	return stdout.Bytes(), nil
}
