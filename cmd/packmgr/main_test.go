package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CDRlease/packmgr/internal/config"
	"github.com/CDRlease/packmgr/internal/githubrelease"
	"github.com/CDRlease/packmgr/internal/manifest"
	"github.com/CDRlease/packmgr/internal/platform"
	"github.com/CDRlease/packmgr/internal/testfixtures"
)

func TestRunPackagesListAndGetUseDefaultPackagesPath(t *testing.T) {
	packagesPath := filepath.Join(t.TempDir(), "packages.json")
	setDefaultPackagesPath(t, packagesPath)

	file := config.NewFile()
	if err := file.AddComponent("server", config.Component{Repo: "CDRlease/tgr_server", Tag: "v0.2.2"}); err != nil {
		t.Fatalf("AddComponent(server) error = %v", err)
	}
	if err := file.AddComponent("engine", config.Component{Repo: "CDRlease/tgr_engine", Tag: "v0.1.1"}); err != nil {
		t.Fatalf("AddComponent(engine) error = %v", err)
	}
	if err := config.SaveFile(packagesPath, file); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"packages", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(packages list) code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if got, want := stdout.String(), "engine repo=CDRlease/tgr_engine tag=v0.1.1\nserver repo=CDRlease/tgr_server tag=v0.2.2\n"; got != want {
		t.Fatalf("list stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("list stderr = %q, want empty", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()

	code = run([]string{"packages", "get", "server", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(packages get) code = %d, want 0; stderr = %q", code, stderr.String())
	}
	const wantJSON = "{\n  \"name\": \"server\",\n  \"repo\": \"CDRlease/tgr_server\",\n  \"tag\": \"v0.2.2\"\n}\n"
	if got := stdout.String(); got != wantJSON {
		t.Fatalf("get stdout = %q, want %q", got, wantJSON)
	}
}

func TestRunPackagesAddCreatesMissingFile(t *testing.T) {
	packagesPath := filepath.Join(t.TempDir(), "packages.json")
	setDefaultPackagesPath(t, packagesPath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"packages", "add", "server", "--repo", "CDRlease/tgr_server", "--tag", "v0.2.2"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(packages add) code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if got, want := stdout.String(), "added component server\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}

	file, err := config.LoadFile(packagesPath)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	component, ok := file.GetComponent("server")
	if !ok {
		t.Fatalf("GetComponent(server) ok = false, want true")
	}
	if component.Repo != "CDRlease/tgr_server" || component.Tag != "v0.2.2" {
		t.Fatalf("component = %#v, want repo/tag preserved", component)
	}
}

func TestRunPackagesUpdateNoop(t *testing.T) {
	packagesPath := filepath.Join(t.TempDir(), "packages.json")
	setDefaultPackagesPath(t, packagesPath)

	file := config.NewFile()
	if err := file.AddComponent("server", config.Component{Repo: "CDRlease/tgr_server", Tag: "v0.2.2"}); err != nil {
		t.Fatalf("AddComponent() error = %v", err)
	}
	if err := config.SaveFile(packagesPath, file); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	original, err := os.ReadFile(packagesPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"packages", "update", "server", "--repo", "CDRlease/tgr_server"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(packages update) code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if got, want := stdout.String(), "component server unchanged\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}

	current, err := os.ReadFile(packagesPath)
	if err != nil {
		t.Fatalf("ReadFile() second error = %v", err)
	}
	if string(current) != string(original) {
		t.Fatalf("packages.json changed after noop update")
	}
}

func TestRunPackagesRemoveDeletesComponent(t *testing.T) {
	packagesPath := filepath.Join(t.TempDir(), "packages.json")
	setDefaultPackagesPath(t, packagesPath)

	file := config.NewFile()
	if err := file.AddComponent("server", config.Component{Repo: "CDRlease/tgr_server", Tag: "v0.2.2"}); err != nil {
		t.Fatalf("AddComponent() error = %v", err)
	}
	if err := config.SaveFile(packagesPath, file); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"packages", "remove", "server"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(packages remove) code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if got, want := stdout.String(), "removed component server\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}

	loaded, err := config.LoadFile(packagesPath)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if _, ok := loaded.GetComponent("server"); ok {
		t.Fatalf("GetComponent(server) ok = true, want false")
	}
}

func TestRunPackagesUsageErrors(t *testing.T) {
	testCases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing packages subcommand",
			args:    []string{"packages"},
			wantErr: "packages subcommand is required",
		},
		{
			name:    "add missing repo",
			args:    []string{"packages", "add", "server", "--tag", "v0.2.2"},
			wantErr: "--repo is required",
		},
		{
			name:    "update missing fields",
			args:    []string{"packages", "update", "server"},
			wantErr: "at least one of --repo or --tag is required",
		},
		{
			name:    "remove missing name",
			args:    []string{"packages", "remove"},
			wantErr: "remove requires exactly 1 component name",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := run(testCase.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("run(%v) code = %d, want 2", testCase.args, code)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if got := stderr.String(); !strings.Contains(got, testCase.wantErr) {
				t.Fatalf("stderr = %q, want substring %q", got, testCase.wantErr)
			}
		})
	}
}

func TestHelpOutputIncludesDetailedSections(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(help) code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Commands:") || !strings.Contains(got, "Examples:") {
		t.Fatalf("help stdout = %q, want detailed sections", got)
	}

	stdout.Reset()
	stderr.Reset()

	code = run([]string{"help", "packages"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(help packages) code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Outputs:") || !strings.Contains(got, "packmgr packages add") {
		t.Fatalf("help packages stdout = %q, want packages detail", got)
	}

	stdout.Reset()
	stderr.Reset()

	code = run([]string{"help", "install"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(help install) code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Install layout:") || !strings.Contains(got, "--dir <path>") {
		t.Fatalf("help install stdout = %q, want install detail", got)
	}
}

func TestRunPackagesAddCheckReleaseSuccess(t *testing.T) {
	server := testfixtures.NewReleaseServer()
	defer server.Close()

	server.AddRelease("CDRlease/tgr_server", "v0.2.2", makeReleaseFixture("server", "v0.2.2",
		manifest.Bundle{
			Name: "server-osx-arm64.zip",
			OS:   "osx",
			Arch: "arm64",
			Validation: manifest.Validation{
				Type:  "bundle-entry-exists",
				Paths: []string{"bin/run.sh"},
			},
		},
		map[string]string{
			"server-osx-arm64.zip": string(testfixtures.BuildZip(map[string]string{
				"bin/run.sh": "#!/usr/bin/env bash\n",
			})),
		},
	))

	packagesPath := filepath.Join(t.TempDir(), "packages.json")
	setDefaultPackagesPath(t, packagesPath)
	setReleaseClientFactory(t, func() *githubrelease.Client {
		return githubrelease.NewClient(githubrelease.Options{
			BaseURL:    server.BaseURL(),
			HTTPClient: server.HTTPClient(),
		})
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"packages", "add", "server", "--repo", "CDRlease/tgr_server", "--tag", "v0.2.2", "--check-release"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(packages add --check-release) code = %d, want 0; stderr = %q", code, stderr.String())
	}

	file, err := config.LoadFile(packagesPath)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if _, ok := file.GetComponent("server"); !ok {
		t.Fatalf("GetComponent(server) ok = false, want true")
	}
}

func TestRunPackagesAddCheckReleaseValidatesLocallyBeforeNetwork(t *testing.T) {
	packagesPath := filepath.Join(t.TempDir(), "packages.json")
	setDefaultPackagesPath(t, packagesPath)
	setReleaseClientFactory(t, func() *githubrelease.Client {
		t.Fatalf("newReleaseClient() should not be called when local validation fails")
		return nil
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"packages", "add", "server", "--repo", "broken", "--tag", "v0.2.2", "--check-release"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run(packages add invalid --check-release) code = %d, want 1", code)
	}
	if got := stderr.String(); !strings.Contains(got, "repo must be in owner/name format") {
		t.Fatalf("stderr = %q, want local validation failure", got)
	}
}

func TestRunPackagesUpdateCheckReleaseFailureDoesNotWriteFile(t *testing.T) {
	server := testfixtures.NewReleaseServer()
	defer server.Close()

	packagesPath := filepath.Join(t.TempDir(), "packages.json")
	setDefaultPackagesPath(t, packagesPath)
	setReleaseClientFactory(t, func() *githubrelease.Client {
		return githubrelease.NewClient(githubrelease.Options{
			BaseURL:    server.BaseURL(),
			HTTPClient: server.HTTPClient(),
		})
	})

	file := config.NewFile()
	if err := file.AddComponent("server", config.Component{Repo: "CDRlease/tgr_server", Tag: "v0.2.2"}); err != nil {
		t.Fatalf("AddComponent() error = %v", err)
	}
	if err := config.SaveFile(packagesPath, file); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	original, err := os.ReadFile(packagesPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"packages", "update", "server", "--tag", "v9.9.9", "--check-release"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run(packages update --check-release) code = %d, want 1", code)
	}
	if got := stderr.String(); !strings.Contains(got, "fetch release CDRlease/tgr_server@v9.9.9") {
		t.Fatalf("stderr = %q, want fetch release failure", got)
	}

	current, err := os.ReadFile(packagesPath)
	if err != nil {
		t.Fatalf("ReadFile() second error = %v", err)
	}
	if string(current) != string(original) {
		t.Fatalf("packages.json changed after failed release validation")
	}
}

func TestRunInstallUsesDefaultPackagesPath(t *testing.T) {
	server := testfixtures.NewReleaseServer()
	defer server.Close()

	server.AddRelease("CDRlease/tgr_server", "v0.2.2", makeReleaseFixture("server", "v0.2.2",
		manifest.Bundle{
			Name: "server-osx-arm64.zip",
			OS:   "osx",
			Arch: "arm64",
			Validation: manifest.Validation{
				Type:  "bundle-entry-exists",
				Paths: []string{"bin/run.sh"},
			},
		},
		map[string]string{
			"server-osx-arm64.zip": string(testfixtures.BuildZip(map[string]string{
				"bin/run.sh": "#!/usr/bin/env bash\n",
			})),
		},
	))

	packagesPath := filepath.Join(t.TempDir(), "packages.json")
	setDefaultPackagesPath(t, packagesPath)
	setReleaseClientFactory(t, func() *githubrelease.Client {
		return githubrelease.NewClient(githubrelease.Options{
			BaseURL:    server.BaseURL(),
			HTTPClient: server.HTTPClient(),
		})
	})
	setPlatformDetector(t, func() (platform.Target, error) {
		return platform.Target{OS: "osx", Arch: "arm64"}, nil
	})

	file := config.NewFile()
	if err := file.AddComponent("server", config.Component{Repo: "CDRlease/tgr_server", Tag: "v0.2.2"}); err != nil {
		t.Fatalf("AddComponent() error = %v", err)
	}
	if err := config.SaveFile(packagesPath, file); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	targetDir := filepath.Join(t.TempDir(), "vendor")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"install", "--dir", targetDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(install) code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(targetDir, "server", "run.sh")); err != nil {
		t.Fatalf("installed run.sh missing: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "packages file : "+packagesPath) {
		t.Fatalf("stdout = %q, want default packages path in output", got)
	}
}

func setDefaultPackagesPath(t *testing.T, path string) {
	t.Helper()
	previous := defaultPackagesPath
	defaultPackagesPath = path
	t.Cleanup(func() {
		defaultPackagesPath = previous
	})
}

func setReleaseClientFactory(t *testing.T, factory func() *githubrelease.Client) {
	t.Helper()
	previous := newReleaseClient
	newReleaseClient = factory
	t.Cleanup(func() {
		newReleaseClient = previous
	})
}

func setPlatformDetector(t *testing.T, detector func() (platform.Target, error)) {
	t.Helper()
	previous := detectPlatform
	detectPlatform = detector
	t.Cleanup(func() {
		detectPlatform = previous
	})
}

func makeReleaseFixture(component, tag string, bundle manifest.Bundle, zips map[string]string) []testfixtures.AssetSpec {
	manifestFile := manifest.File{
		SchemaVersion: 1,
		Mode:          "release",
		Component:     component,
		SourceRepo:    "CDRlease/" + component,
		Tag:           tag,
		CommitSHA:     "abc123",
		BuiltAt:       "2026-04-14T00:00:00Z",
		Bundles:       []manifest.Bundle{bundle},
	}

	manifestBytes, err := json.Marshal(manifestFile)
	if err != nil {
		panic(err)
	}

	assets := []testfixtures.AssetSpec{
		{Name: "manifest.json", Content: manifestBytes},
	}

	checksumLines := []string{
		fmt.Sprintf("%s  manifest.json", sha256Hex(manifestBytes)),
	}
	for name, content := range zips {
		zipBytes := []byte(content)
		assets = append(assets, testfixtures.AssetSpec{Name: name, Content: zipBytes})
		checksumLines = append(checksumLines, fmt.Sprintf("%s  %s", sha256Hex(zipBytes), name))
	}
	assets = append(assets, testfixtures.AssetSpec{
		Name:    "SHA256SUMS.txt",
		Content: []byte(strings.Join(checksumLines, "\n") + "\n"),
	})

	return assets
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
