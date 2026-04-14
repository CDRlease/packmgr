package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/CDRlease/packmgr/internal/platform"
)

type File struct {
	SchemaVersion int      `json:"schemaVersion"`
	Mode          string   `json:"mode"`
	Component     string   `json:"component"`
	SourceRepo    string   `json:"sourceRepo"`
	Tag           string   `json:"tag"`
	CommitSHA     string   `json:"commitSha"`
	BuiltAt       string   `json:"builtAt"`
	Bundles       []Bundle `json:"bundles"`
}

type Bundle struct {
	Name       string     `json:"name"`
	OS         string     `json:"os"`
	Arch       string     `json:"arch"`
	Validation Validation `json:"validation"`
}

type Validation struct {
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

func Parse(data []byte) (File, error) {
	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return File{}, fmt.Errorf("parse manifest.json: %w", err)
	}
	return file, nil
}

func (f File) ValidateForComponent(component string) error {
	if f.Mode != "release" {
		return fmt.Errorf("manifest mode must be release")
	}
	if f.Component != component {
		return fmt.Errorf("manifest component mismatch: want %s, got %s", component, f.Component)
	}
	if len(f.Bundles) == 0 {
		return fmt.Errorf("manifest bundles must not be empty")
	}
	for _, bundle := range f.Bundles {
		if err := validateBundle(bundle); err != nil {
			return err
		}
	}
	return nil
}

func SelectBundle(file File, target platform.Target) (Bundle, error) {
	var exact []Bundle
	var fallback []Bundle

	for _, bundle := range file.Bundles {
		switch {
		case bundle.OS == target.OS && bundle.Arch == target.Arch:
			exact = append(exact, bundle)
		case bundle.OS == "any" && bundle.Arch == "any":
			fallback = append(fallback, bundle)
		}
	}

	if len(exact) > 1 {
		return Bundle{}, fmt.Errorf("multiple exact bundles found for %s-%s", target.OS, target.Arch)
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(fallback) > 1 {
		return Bundle{}, fmt.Errorf("multiple any-any bundles found")
	}
	if len(fallback) == 1 {
		return fallback[0], nil
	}
	return Bundle{}, fmt.Errorf("no compatible bundle found for %s-%s", target.OS, target.Arch)
}

func ValidateChecksums(checksums []byte, files map[string]string) error {
	entries := parseChecksums(checksums)
	for name, filePath := range files {
		expected, ok := entries[name]
		if !ok {
			return fmt.Errorf("checksum entry not found for %s", name)
		}
		actual, err := checksumFile(filePath)
		if err != nil {
			return err
		}
		if !strings.EqualFold(actual, expected) {
			return fmt.Errorf("checksum mismatch for %s", name)
		}
	}
	return nil
}

func StripPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	first := firstSegment(paths[0])
	if first == "" {
		return ""
	}
	for _, current := range paths[1:] {
		if firstSegment(current) != first {
			return ""
		}
	}
	return first
}

func TrimValidationPaths(bundle Bundle) []string {
	prefix := StripPrefix(bundle.Validation.Paths)
	result := make([]string, 0, len(bundle.Validation.Paths))
	for _, current := range bundle.Validation.Paths {
		trimmed := trimPrefixPath(current, prefix)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func validateBundle(bundle Bundle) error {
	if strings.TrimSpace(bundle.Name) == "" {
		return fmt.Errorf("bundle name must not be empty")
	}
	if bundle.Validation.Type != "bundle-entry-exists" {
		return fmt.Errorf("unsupported validation type %q", bundle.Validation.Type)
	}
	if len(bundle.Validation.Paths) == 0 {
		return fmt.Errorf("bundle %s validation paths must not be empty", bundle.Name)
	}

	if bundle.OS == "any" || bundle.Arch == "any" {
		if bundle.OS != "any" || bundle.Arch != "any" {
			return fmt.Errorf("bundle %s must use any-any together", bundle.Name)
		}
		return nil
	}

	switch bundle.OS {
	case "linux", "osx", "win":
	default:
		return fmt.Errorf("unsupported bundle OS %q", bundle.OS)
	}
	switch bundle.Arch {
	case "amd64", "arm64":
	default:
		return fmt.Errorf("unsupported bundle architecture %q", bundle.Arch)
	}

	return nil
}

func parseChecksums(contents []byte) map[string]string {
	entries := make(map[string]string)
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		entries[name] = fields[0]
	}
	return entries
}

func checksumFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", filePath, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func firstSegment(current string) string {
	cleaned := path.Clean(strings.ReplaceAll(current, "\\", "/"))
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return ""
	}
	return strings.Split(cleaned, "/")[0]
}

func trimPrefixPath(current, prefix string) string {
	cleaned := path.Clean(strings.ReplaceAll(current, "\\", "/"))
	if prefix == "" {
		return cleaned
	}
	if cleaned == prefix {
		return ""
	}
	return strings.TrimPrefix(cleaned, prefix+"/")
}
