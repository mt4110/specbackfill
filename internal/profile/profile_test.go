package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectSkipsHeavyDependencyDirs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeProfileTestFile(t, filepath.Join(root, "node_modules", "package", "openapi.yaml"), "openapi: 3.0.0\n")

	profile, err := Detect(root)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if profile.OpenAPI {
		t.Fatalf("Detect().OpenAPI = true, want false for skipped dependency dir")
	}
}

func writeProfileTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
