package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/CDRlease/packmgr/internal/platform"
)

func TestSelectBundlePrefersExactMatch(t *testing.T) {
	t.Parallel()

	file := File{
		Mode:      "release",
		Component: "server",
		Bundles: []Bundle{
			{Name: "server-any-any.zip", OS: "any", Arch: "any", Validation: Validation{Type: "bundle-entry-exists", Paths: []string{"bin/run.sh"}}},
			{Name: "server-osx-arm64.zip", OS: "osx", Arch: "arm64", Validation: Validation{Type: "bundle-entry-exists", Paths: []string{"bin/run.sh"}}},
		},
	}

	bundle, err := SelectBundle(file, platform.Target{OS: "osx", Arch: "arm64"})
	if err != nil {
		t.Fatalf("SelectBundle() error = %v", err)
	}
	if bundle.Name != "server-osx-arm64.zip" {
		t.Fatalf("SelectBundle() = %q, want exact bundle", bundle.Name)
	}
}

func TestSelectBundleFallsBackToAnyAny(t *testing.T) {
	t.Parallel()

	file := File{
		Mode:      "release",
		Component: "engine",
		Bundles: []Bundle{
			{Name: "engine-any-any.zip", OS: "any", Arch: "any", Validation: Validation{Type: "bundle-entry-exists", Paths: []string{"bin/lockstep.engine.dll"}}},
		},
	}

	bundle, err := SelectBundle(file, platform.Target{OS: "linux", Arch: "amd64"})
	if err != nil {
		t.Fatalf("SelectBundle() error = %v", err)
	}
	if bundle.Name != "engine-any-any.zip" {
		t.Fatalf("SelectBundle() = %q, want any-any bundle", bundle.Name)
	}
}

func TestSelectBundleRejectsAmbiguousOrMissingMatches(t *testing.T) {
	t.Parallel()

	ambiguous := File{
		Bundles: []Bundle{
			{Name: "a.zip", OS: "osx", Arch: "arm64", Validation: Validation{Type: "bundle-entry-exists", Paths: []string{"bin/a"}}},
			{Name: "b.zip", OS: "osx", Arch: "arm64", Validation: Validation{Type: "bundle-entry-exists", Paths: []string{"bin/b"}}},
		},
	}
	if _, err := SelectBundle(ambiguous, platform.Target{OS: "osx", Arch: "arm64"}); err == nil {
		t.Fatalf("SelectBundle() error = nil, want ambiguity error")
	}

	missing := File{
		Bundles: []Bundle{
			{Name: "server-linux-amd64.zip", OS: "linux", Arch: "amd64", Validation: Validation{Type: "bundle-entry-exists", Paths: []string{"bin/run.sh"}}},
		},
	}
	if _, err := SelectBundle(missing, platform.Target{OS: "osx", Arch: "arm64"}); err == nil {
		t.Fatalf("SelectBundle() error = nil, want missing bundle error")
	}
}

func TestValidateChecksums(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	zipPath := filepath.Join(dir, "engine-any-any.zip")

	if err := os.WriteFile(manifestPath, []byte("manifest"), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	if err := os.WriteFile(zipPath, []byte("zip"), 0o644); err != nil {
		t.Fatalf("WriteFile(zip) error = %v", err)
	}

	checksums := []byte(
		hexFor([]byte("manifest")) + "  manifest.json\n" +
			hexFor([]byte("zip")) + "  engine-any-any.zip\n",
	)
	if err := ValidateChecksums(checksums, map[string]string{
		"manifest.json":      manifestPath,
		"engine-any-any.zip": zipPath,
	}); err != nil {
		t.Fatalf("ValidateChecksums() error = %v", err)
	}
}

func TestValidateChecksumsMismatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	zipPath := filepath.Join(dir, "engine-any-any.zip")

	if err := os.WriteFile(manifestPath, []byte("manifest"), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	if err := os.WriteFile(zipPath, []byte("zip"), 0o644); err != nil {
		t.Fatalf("WriteFile(zip) error = %v", err)
	}

	checksums := []byte("deadbeef  manifest.json\nbeadfeed  engine-any-any.zip\n")
	if err := ValidateChecksums(checksums, map[string]string{
		"manifest.json":      manifestPath,
		"engine-any-any.zip": zipPath,
	}); err == nil {
		t.Fatalf("ValidateChecksums() error = nil, want mismatch because checksums are fake")
	}
}

func TestValidateChecksumsMissingEntry(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(filePath, []byte("manifest"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := ValidateChecksums([]byte("abcd  other.txt\n"), map[string]string{"manifest.json": filePath}); err == nil {
		t.Fatalf("ValidateChecksums() error = nil, want missing entry error")
	}
}

func TestStripPrefixAndTrimValidationPaths(t *testing.T) {
	t.Parallel()

	if got := StripPrefix([]string{"bin/run.sh", "bin/mesh/mesh"}); got != "bin" {
		t.Fatalf("StripPrefix(bin) = %q, want %q", got, "bin")
	}
	if got := StripPrefix([]string{"codegen-osx-arm64/scripts/gen.sh", "codegen-osx-arm64/Config/HashPrimes.json"}); got != "codegen-osx-arm64" {
		t.Fatalf("StripPrefix(codegen) = %q, want codegen-osx-arm64", got)
	}
	if got := StripPrefix([]string{"README.md", "LICENSE"}); got != "" {
		t.Fatalf("StripPrefix(root files) = %q, want empty", got)
	}

	bundle := Bundle{
		Name: "server-osx-arm64.zip",
		OS:   "osx",
		Arch: "arm64",
		Validation: Validation{
			Type:  "bundle-entry-exists",
			Paths: []string{"bin/run.sh", "bin/mesh/mesh"},
		},
	}
	trimmed := TrimValidationPaths(bundle)
	if len(trimmed) != 2 || trimmed[0] != "run.sh" || trimmed[1] != "mesh/mesh" {
		t.Fatalf("TrimValidationPaths() = %#v, want stripped paths", trimmed)
	}
}

func TestSelectBundleRejectsDuplicateFallbacks(t *testing.T) {
	t.Parallel()

	file := File{
		Bundles: []Bundle{
			{Name: "engine-any-any-a.zip", OS: "any", Arch: "any", Validation: Validation{Type: "bundle-entry-exists", Paths: []string{"bin/a"}}},
			{Name: "engine-any-any-b.zip", OS: "any", Arch: "any", Validation: Validation{Type: "bundle-entry-exists", Paths: []string{"bin/b"}}},
		},
	}
	if _, err := SelectBundle(file, platform.Target{OS: "linux", Arch: "amd64"}); err == nil {
		t.Fatalf("SelectBundle() error = nil, want duplicate any-any error")
	}
}

func TestValidateForComponent(t *testing.T) {
	t.Parallel()

	file := File{
		Mode:      "release",
		Component: "config",
		Bundles: []Bundle{
			{Name: "config-any-any.zip", OS: "any", Arch: "any", Validation: Validation{Type: "bundle-entry-exists", Paths: []string{"bin/Luban.dll"}}},
		},
	}
	if err := file.ValidateForComponent("config"); err != nil {
		t.Fatalf("ValidateForComponent() error = %v", err)
	}

	file.Mode = "smoke"
	if err := file.ValidateForComponent("config"); err == nil {
		t.Fatalf("ValidateForComponent() error = nil, want mode error")
	}
}

func hexFor(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
