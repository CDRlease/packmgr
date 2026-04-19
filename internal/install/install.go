package install

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/CDRlease/packmgr/internal/config"
	"github.com/CDRlease/packmgr/internal/githubrelease"
	"github.com/CDRlease/packmgr/internal/manifest"
	"github.com/CDRlease/packmgr/internal/platform"
)

type Manager struct {
	client *githubrelease.Client
	log    io.Writer
}

type InstallOptions struct {
	ForceDownload bool
}

type BundleInstallOptions struct {
	ComponentName string
	TargetRoot    string
	ZipPath       string
	ManifestPath  string
	ChecksumsPath string
	Bundle        manifest.Bundle
}

func NewManager(client *githubrelease.Client, log io.Writer) *Manager {
	return &Manager{client: client, log: log}
}

func (m *Manager) Install(ctx context.Context, lockFile config.File, targetRoot string, target platform.Target, options InstallOptions) error {
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return fmt.Errorf("create target root: %w", err)
	}

	components := lockFile.SortedComponents()
	installed := 0
	skipped := 0
	for index, component := range components {
		fmt.Fprintf(m.log, "[%d/%d] %s\n", index+1, len(components), component.Name)
		didInstall, err := m.installComponent(ctx, component, targetRoot, target, options)
		if err != nil {
			return err
		}
		fmt.Fprintln(m.log)
		if didInstall {
			installed++
			continue
		}
		skipped++
	}

	fmt.Fprintln(m.log, "Done.")
	fmt.Fprintf(m.log, "Installed: %d\n", installed)
	fmt.Fprintf(m.log, "Skipped  : %d\n", skipped)
	fmt.Fprintln(m.log, "Failed   : 0")
	return nil
}

func (m *Manager) installComponent(ctx context.Context, component config.ResolvedComponent, targetRoot string, target platform.Target, options InstallOptions) (bool, error) {
	fmt.Fprintf(m.log, "  repo           : %s\n", component.Repo)
	fmt.Fprintf(m.log, "  version        : %s\n", component.Tag)

	var release *githubrelease.Release
	targetTag := component.Tag
	if config.IsLatestTag(component.Tag) {
		resolvedRelease, err := m.fetchRelease(ctx, component)
		if err != nil {
			return false, err
		}
		release = resolvedRelease
		targetTag = release.TagName
		fmt.Fprintf(m.log, "  resolved tag   : %s\n", targetTag)
	}

	componentDir := filepath.Join(targetRoot, component.Name)
	if options.ForceDownload {
		fmt.Fprintln(m.log, "  cache          : bypass (--force-download)")
	} else if installedBundle, reason, ok := installedComponentCacheHit(component, componentDir, target, targetTag); ok {
		fmt.Fprintf(m.log, "  selected bundle: %s\n", installedBundle.Name)
		fmt.Fprintln(m.log, "  cache          : hit")
		fmt.Fprintln(m.log, "  download       : skipped")
		fmt.Fprintf(m.log, "  install dir    : %s\n", componentDir)
		fmt.Fprintln(m.log, "  extract        : skipped")
		return false, nil
	} else {
		fmt.Fprintf(m.log, "  cache          : miss (%s)\n", reason)
	}

	if release == nil {
		fetchedRelease, err := m.fetchRelease(ctx, component)
		if err != nil {
			return false, err
		}
		release = fetchedRelease
	}

	manifestAsset, ok := release.FindAsset("manifest.json")
	if !ok {
		return false, fmt.Errorf("release %s@%s is missing manifest.json", component.Repo, component.Tag)
	}
	checksumAsset, ok := release.FindAsset("SHA256SUMS.txt")
	if !ok {
		return false, fmt.Errorf("release %s@%s is missing SHA256SUMS.txt", component.Repo, component.Tag)
	}

	workDir, err := os.MkdirTemp("", "packmgr-"+component.Name+"-")
	if err != nil {
		return false, fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	manifestPath := filepath.Join(workDir, "manifest.json")
	checksumPath := filepath.Join(workDir, "SHA256SUMS.txt")

	if err := m.client.DownloadAsset(ctx, manifestAsset, manifestPath); err != nil {
		return false, err
	}
	if err := m.client.DownloadAsset(ctx, checksumAsset, checksumPath); err != nil {
		return false, err
	}

	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return false, fmt.Errorf("read manifest.json: %w", err)
	}

	manifestFile, err := manifest.Parse(manifestBytes)
	if err != nil {
		return false, err
	}
	if err := manifestFile.ValidateForComponent(component.Name); err != nil {
		return false, err
	}

	bundle, err := manifest.SelectBundle(manifestFile, target)
	if err != nil {
		return false, err
	}
	fmt.Fprintf(m.log, "  selected bundle: %s\n", bundle.Name)

	bundleAsset, ok := release.FindAsset(bundle.Name)
	if !ok {
		return false, fmt.Errorf("release %s@%s is missing %s", component.Repo, component.Tag, bundle.Name)
	}

	zipPath := filepath.Join(workDir, bundle.Name)
	if err := m.client.DownloadAsset(ctx, bundleAsset, zipPath); err != nil {
		return false, err
	}
	fmt.Fprintln(m.log, "  download       : ok")

	checksumBytes, err := os.ReadFile(checksumPath)
	if err != nil {
		return false, fmt.Errorf("read SHA256SUMS.txt: %w", err)
	}
	if err := manifest.ValidateChecksums(checksumBytes, map[string]string{
		bundle.Name:     zipPath,
		"manifest.json": manifestPath,
	}); err != nil {
		return false, err
	}
	fmt.Fprintln(m.log, "  checksum       : ok")

	if err := InstallBundle(BundleInstallOptions{
		ComponentName: component.Name,
		TargetRoot:    targetRoot,
		ZipPath:       zipPath,
		ManifestPath:  manifestPath,
		ChecksumsPath: checksumPath,
		Bundle:        bundle,
	}); err != nil {
		return false, err
	}

	fmt.Fprintf(m.log, "  install dir    : %s\n", componentDir)
	fmt.Fprintln(m.log, "  extract        : ok")
	return true, nil
}

func (m *Manager) fetchRelease(ctx context.Context, component config.ResolvedComponent) (*githubrelease.Release, error) {
	if config.IsLatestTag(component.Tag) {
		return m.client.FetchLatestRelease(ctx, component.Repo)
	}
	return m.client.FetchRelease(ctx, component.Repo, component.Tag)
}

func installedComponentCacheHit(component config.ResolvedComponent, componentDir string, target platform.Target, targetTag string) (manifest.Bundle, string, bool) {
	if _, err := os.Stat(componentDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return manifest.Bundle{}, "not installed", false
		}
		return manifest.Bundle{}, fmt.Sprintf("stat install dir: %v", err), false
	}

	installedManifest, installedBundle, err := loadInstalledComponent(component.Name, componentDir, target)
	if err != nil {
		return manifest.Bundle{}, err.Error(), false
	}
	if installedManifest.Tag != targetTag {
		return manifest.Bundle{}, fmt.Sprintf("installed tag %s does not match target %s", installedManifest.Tag, targetTag), false
	}
	return installedBundle, "", true
}

func loadInstalledComponent(componentName, componentDir string, target platform.Target) (manifest.File, manifest.Bundle, error) {
	manifestPath := filepath.Join(componentDir, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return manifest.File{}, manifest.Bundle{}, fmt.Errorf("installed manifest.json not found")
		}
		return manifest.File{}, manifest.Bundle{}, fmt.Errorf("read installed manifest.json: %w", err)
	}

	manifestFile, err := manifest.Parse(manifestBytes)
	if err != nil {
		return manifest.File{}, manifest.Bundle{}, err
	}
	if err := manifestFile.ValidateForComponent(componentName); err != nil {
		return manifest.File{}, manifest.Bundle{}, err
	}

	bundle, err := manifest.SelectBundle(manifestFile, target)
	if err != nil {
		return manifest.File{}, manifest.Bundle{}, err
	}
	if err := validateInstalledFiles(componentDir, bundle); err != nil {
		return manifest.File{}, manifest.Bundle{}, err
	}
	return manifestFile, bundle, nil
}

func InstallBundle(options BundleInstallOptions) error {
	stagingDir, err := os.MkdirTemp(options.TargetRoot, "."+options.ComponentName+".staging-")
	if err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}

	cleanupStaging := true
	defer func() {
		if cleanupStaging {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	stripPrefix := manifest.StripPrefix(options.Bundle.Validation.Paths)
	if err := extractZip(options.ZipPath, stagingDir, stripPrefix); err != nil {
		return err
	}
	if err := copyFile(options.ManifestPath, filepath.Join(stagingDir, "manifest.json")); err != nil {
		return err
	}
	if err := copyFile(options.ChecksumsPath, filepath.Join(stagingDir, "SHA256SUMS.txt")); err != nil {
		return err
	}
	if err := validateInstalledFiles(stagingDir, options.Bundle); err != nil {
		return err
	}

	componentDir := filepath.Join(options.TargetRoot, options.ComponentName)
	backupDir := ""
	if _, err := os.Stat(componentDir); err == nil {
		backupDir = filepath.Join(options.TargetRoot, "."+options.ComponentName+".backup-"+filepath.Base(stagingDir))
		if err := os.Rename(componentDir, backupDir); err != nil {
			return fmt.Errorf("move current component directory: %w", err)
		}
	}

	if err := os.Rename(stagingDir, componentDir); err != nil {
		if backupDir != "" {
			_ = os.Rename(backupDir, componentDir)
		}
		return fmt.Errorf("activate staging directory: %w", err)
	}
	cleanupStaging = false

	if backupDir != "" {
		if err := os.RemoveAll(backupDir); err != nil {
			return fmt.Errorf("remove backup directory: %w", err)
		}
	}
	return nil
}

func validateInstalledFiles(componentDir string, bundle manifest.Bundle) error {
	for _, current := range manifest.TrimValidationPaths(bundle) {
		filePath := filepath.Join(componentDir, filepath.FromSlash(current))
		info, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("expected installed file %s: %w", current, err)
		}
		if info.IsDir() {
			return fmt.Errorf("expected file but found directory: %s", current)
		}
	}
	return nil
}

func extractZip(zipPath, destination, stripPrefix string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		relativePath, skip, err := rewrittenPath(file.Name, stripPrefix)
		if err != nil {
			return err
		}
		if skip {
			continue
		}

		targetPath := filepath.Join(destination, filepath.FromSlash(relativePath))
		if err := ensureWithinRoot(destination, targetPath); err != nil {
			return err
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create directory %s: %w", targetPath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create parent directory for %s: %w", targetPath, err)
		}

		input, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", file.Name, err)
		}

		mode := file.Mode()
		if mode == 0 {
			mode = 0o644
		}

		output, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			input.Close()
			return fmt.Errorf("create extracted file %s: %w", targetPath, err)
		}

		if _, err := io.Copy(output, input); err != nil {
			output.Close()
			input.Close()
			return fmt.Errorf("extract %s: %w", file.Name, err)
		}

		if err := output.Close(); err != nil {
			input.Close()
			return fmt.Errorf("close extracted file %s: %w", targetPath, err)
		}
		if err := input.Close(); err != nil {
			return fmt.Errorf("close zip entry %s: %w", file.Name, err)
		}
	}

	return nil
}

func rewrittenPath(entryName, stripPrefix string) (string, bool, error) {
	normalized := strings.ReplaceAll(entryName, "\\", "/")
	if strings.HasPrefix(normalized, "/") || isWindowsAbsolute(normalized) {
		return "", false, fmt.Errorf("unsafe zip entry path: %s", entryName)
	}

	if stripPrefix != "" {
		switch {
		case normalized == stripPrefix, normalized == stripPrefix+"/":
			return "", true, nil
		case strings.HasPrefix(normalized, stripPrefix+"/"):
			normalized = strings.TrimPrefix(normalized, stripPrefix+"/")
		}
	}

	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == "" {
		return "", true, nil
	}
	if strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", false, fmt.Errorf("unsafe zip entry path: %s", entryName)
	}
	return cleaned, false, nil
}

func ensureWithinRoot(root, target string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	prefix := rootAbs + string(os.PathSeparator)
	if targetAbs != rootAbs && !strings.HasPrefix(targetAbs, prefix) {
		return fmt.Errorf("unsafe zip entry path: %s", target)
	}
	return nil
}

func isWindowsAbsolute(path string) bool {
	return len(path) >= 3 && path[1] == ':' && (path[2] == '/' || path[2] == '\\')
}

func copyFile(source, destination string) error {
	input, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read %s: %w", source, err)
	}
	if err := os.WriteFile(destination, input, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", destination, err)
	}
	return nil
}
