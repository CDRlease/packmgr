package install

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/CDRlease/packmgr/internal/manifest"
)

func TestInstallBundleReplacesExistingDirectory(t *testing.T) {
	t.Parallel()

	targetRoot := t.TempDir()
	componentDir := filepath.Join(targetRoot, "server")
	if err := os.MkdirAll(componentDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(componentDir, "stale.txt"), []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile(stale) error = %v", err)
	}

	zipPath := filepath.Join(t.TempDir(), "server-osx-arm64.zip")
	writeZip(t, zipPath, map[string]string{
		"bin/run.sh":    "#!/usr/bin/env bash\n",
		"bin/mesh/mesh": "mesh binary",
	})

	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	checksumsPath := filepath.Join(t.TempDir(), "SHA256SUMS.txt")
	if err := os.WriteFile(manifestPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	if err := os.WriteFile(checksumsPath, []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(checksums) error = %v", err)
	}

	err := InstallBundle(BundleInstallOptions{
		ComponentName: "server",
		TargetRoot:    targetRoot,
		ZipPath:       zipPath,
		ManifestPath:  manifestPath,
		ChecksumsPath: checksumsPath,
		Bundle: manifest.Bundle{
			Name: "server-osx-arm64.zip",
			OS:   "osx",
			Arch: "arm64",
			Validation: manifest.Validation{
				Type:  "bundle-entry-exists",
				Paths: []string{"bin/run.sh", "bin/mesh/mesh"},
			},
		},
	})
	if err != nil {
		t.Fatalf("InstallBundle() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(componentDir, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale file still exists after replacement")
	}
	assertFileExists(t, filepath.Join(componentDir, "run.sh"))
	assertFileExists(t, filepath.Join(componentDir, "mesh", "mesh"))
	assertFileExists(t, filepath.Join(componentDir, "manifest.json"))
	assertFileExists(t, filepath.Join(componentDir, "SHA256SUMS.txt"))
}

func TestExtractZipRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	zipPath := filepath.Join(t.TempDir(), "bad.zip")
	writeZip(t, zipPath, map[string]string{"../evil": "boom"})

	if err := extractZip(zipPath, t.TempDir(), ""); err == nil {
		t.Fatalf("extractZip() error = nil, want path traversal rejection")
	}
}

func TestExtractZipRejectsAbsolutePath(t *testing.T) {
	t.Parallel()

	zipPath := filepath.Join(t.TempDir(), "absolute.zip")
	writeZip(t, zipPath, map[string]string{"/tmp/evil": "boom"})

	if err := extractZip(zipPath, t.TempDir(), ""); err == nil {
		t.Fatalf("extractZip() error = nil, want absolute path rejection")
	}
}

func writeZip(t *testing.T, zipPath string, entries map[string]string) {
	t.Helper()

	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Create(%s) error = %v", zipPath, err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	for name, content := range entries {
		entryWriter, err := writer.Create(name)
		if err != nil {
			t.Fatalf("Create(%s) error = %v", name, err)
		}
		if _, err := entryWriter.Write([]byte(content)); err != nil {
			t.Fatalf("Write(%s) error = %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func assertFileExists(t *testing.T, filePath string) {
	t.Helper()
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("expected file %s: %v", filePath, err)
	}
}
