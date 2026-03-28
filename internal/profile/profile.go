package profile

import (
	"io/fs"
	"os"
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

	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}

		normalized := filepath.ToSlash(path)
		switch {
		case strings.HasSuffix(normalized, ".proto"):
			profile.Proto = true
		case strings.HasSuffix(normalized, "/openapi.yaml"),
			strings.HasSuffix(normalized, "/openapi.yml"),
			strings.Contains(normalized, "/openapi/") && (strings.HasSuffix(normalized, ".yaml") || strings.HasSuffix(normalized, ".yml")),
			filepath.Base(normalized) == "openapi.yaml",
			filepath.Base(normalized) == "openapi.yml":
			profile.OpenAPI = true
		}
		return nil
	}); err != nil {
		return model.RepoProfile{}, err
	}

	return profile, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
