package profile

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mt4110/specbackfill/internal/model"
)

func Detect(root string) (model.RepoProfile, error) {
	profile := model.RepoProfile{
		Go:         exists(filepath.Join(root, "go.mod")),
		Node:       exists(filepath.Join(root, "package.json")) || exists(filepath.Join(root, "tsconfig.json")),
		Prisma:     exists(filepath.Join(root, "prisma", "schema.prisma")),
		Migrations: exists(filepath.Join(root, "migrations")) || exists(filepath.Join(root, "db", "migrations")) || exists(filepath.Join(root, "prisma", "migrations")),
	}

	if err := filepath.WalkDir(root, func(currentPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && shouldSkipProfileDir(entry.Name()) {
			return filepath.SkipDir
		}

		normalized := filepath.ToSlash(currentPath)
		switch {
		case strings.HasSuffix(normalized, ".proto"):
			profile.Proto = true
		case strings.HasSuffix(normalized, "/openapi.yaml"),
			strings.HasSuffix(normalized, "/openapi.yml"),
			strings.Contains(normalized, "/openapi/") && (strings.HasSuffix(normalized, ".yaml") || strings.HasSuffix(normalized, ".yml")),
			path.Base(normalized) == "openapi.yaml",
			path.Base(normalized) == "openapi.yml":
			profile.OpenAPI = true
		}
		return nil
	}); err != nil {
		return model.RepoProfile{}, err
	}

	return profile, nil
}

func shouldSkipProfileDir(name string) bool {
	switch name {
	case ".git", ".next", "build", "dist", "node_modules", "vendor":
		return true
	default:
		return false
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
