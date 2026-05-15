package diffinput

import (
	"context"
	"fmt"

	"github.com/mt4110/specbackfill/internal/diffsrc"
)

type Selection struct {
	Base     string
	Head     string
	DiffFile string
}

func Validate(selection Selection) error {
	if selection.DiffFile != "" && (selection.Base != "" || selection.Head != "") {
		return fmt.Errorf("--diff-file cannot be combined with --base/--head")
	}
	if (selection.Base == "") != (selection.Head == "") {
		return fmt.Errorf("--base and --head must be provided together")
	}
	return nil
}

func Kind(selection Selection) string {
	if selection.DiffFile != "" {
		return "diff_file"
	}
	if selection.Base != "" && selection.Head != "" {
		return "range"
	}
	return "working_tree"
}

func Summary(selection Selection) string {
	if selection.DiffFile != "" {
		return "diff file"
	}
	if selection.Base != "" && selection.Head != "" {
		return fmt.Sprintf("git range diff (%s..%s)", selection.Base, selection.Head)
	}
	return "working tree diff (tracked changes)"
}

func Notes(selection Selection) []string {
	switch {
	case selection.DiffFile != "":
		return []string{"only the provided unified diff file was evaluated"}
	case selection.Base != "" && selection.Head != "":
		return []string{"working tree changes are not included in --base/--head mode"}
	default:
		return []string{"untracked files are not included unless staged with git add -N"}
	}
}

func ResolveRoots(ctx context.Context, cwd string, selection Selection) (string, string, error) {
	repoRoot, err := diffsrc.RepoRoot(ctx, cwd)
	if err != nil {
		if selection.DiffFile != "" {
			return cwd, cwd, nil
		}
		return "", "", err
	}

	if selection.DiffFile != "" {
		return cwd, repoRoot, nil
	}
	return repoRoot, repoRoot, nil
}

func Load(ctx context.Context, cwd string, selection Selection) ([]byte, error) {
	switch {
	case selection.DiffFile != "":
		return diffsrc.DiffFile(cwd, selection.DiffFile)
	case selection.Base != "" && selection.Head != "":
		return diffsrc.GitRange(ctx, cwd, selection.Base, selection.Head)
	default:
		return diffsrc.WorkingTree(ctx, cwd)
	}
}
