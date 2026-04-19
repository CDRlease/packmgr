package install

import (
	"bytes"
	"context"
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

func TestManagerInstallsFlattenedComponentsFromReleaseServer(t *testing.T) {
	t.Parallel()

	server := testfixtures.NewReleaseServer()
	defer server.Close()

	serverRelease := makeReleaseFixture("server", "v0.2.2",
		manifest.Bundle{
			Name: "server-osx-arm64.zip",
			OS:   "osx",
			Arch: "arm64",
			Validation: manifest.Validation{
				Type: "bundle-entry-exists",
				Paths: []string{
					"bin/run.sh",
					"bin/mesh/mesh",
					"bin/docker/docker-compose.yaml",
				},
			},
		},
		map[string]string{
			"server-osx-arm64.zip": string(testfixtures.BuildZip(map[string]string{
				"bin/run.sh":                     "#!/usr/bin/env bash\n",
				"bin/mesh/mesh":                  "mesh binary",
				"bin/docker/docker-compose.yaml": "services:\n",
			})),
		},
	)
	engineRelease := makeReleaseFixture("engine", "v0.1.1",
		manifest.Bundle{
			Name: "engine-any-any.zip",
			OS:   "any",
			Arch: "any",
			Validation: manifest.Validation{
				Type:  "bundle-entry-exists",
				Paths: []string{"bin/lockstep.engine.dll"},
			},
		},
		map[string]string{
			"engine-any-any.zip": string(testfixtures.BuildZip(map[string]string{
				"bin/lockstep.engine.dll": "engine dll",
			})),
		},
	)
	configRelease := makeReleaseFixture("config", "v0.1.1",
		manifest.Bundle{
			Name: "config-any-any.zip",
			OS:   "any",
			Arch: "any",
			Validation: manifest.Validation{
				Type:  "bundle-entry-exists",
				Paths: []string{"bin/Luban.dll", "bin/gen.sh"},
			},
		},
		map[string]string{
			"config-any-any.zip": string(testfixtures.BuildZip(map[string]string{
				"bin/Luban.dll": "luban",
				"bin/gen.sh":    "#!/usr/bin/env bash\n",
			})),
		},
	)
	codegenRelease := makeReleaseFixture("codegen", "v0.4.4",
		manifest.Bundle{
			Name: "codegen-osx-arm64.zip",
			OS:   "osx",
			Arch: "arm64",
			Validation: manifest.Validation{
				Type:  "bundle-entry-exists",
				Paths: []string{"codegen-osx-arm64/lockstep.ecs.generator.dll", "codegen-osx-arm64/Config/HashPrimes.json", "codegen-osx-arm64/scripts/gen.sh"},
			},
		},
		map[string]string{
			"codegen-osx-arm64.zip": string(testfixtures.BuildZip(map[string]string{
				"codegen-osx-arm64/lockstep.ecs.generator.dll": "generator",
				"codegen-osx-arm64/Config/HashPrimes.json":     "{}",
				"codegen-osx-arm64/scripts/gen.sh":             "#!/usr/bin/env bash\n",
			})),
		},
	)

	server.AddRelease("CDRlease/tgr_server", "v0.2.2", serverRelease)
	server.AddRelease("CDRlease/tgr_engine", "v0.1.1", engineRelease)
	server.AddRelease("CDRlease/tgr_config", "v0.1.1", configRelease)
	server.AddRelease("CDRlease/tgr_codegen", "v0.4.4", codegenRelease)

	lockFile := config.File{
		SchemaVersion: 1,
		Components: map[string]config.Component{
			"server":  {Repo: "CDRlease/tgr_server", Tag: "v0.2.2"},
			"engine":  {Repo: "CDRlease/tgr_engine", Tag: "v0.1.1"},
			"config":  {Repo: "CDRlease/tgr_config", Tag: "v0.1.1"},
			"codegen": {Repo: "CDRlease/tgr_codegen", Tag: "v0.4.4"},
		},
	}

	client := githubrelease.NewClient(githubrelease.Options{
		BaseURL:    server.BaseURL(),
		HTTPClient: server.HTTPClient(),
	})

	targetRoot := t.TempDir()
	manager := NewManager(client, ioDiscard{})
	if err := manager.Install(context.Background(), lockFile, targetRoot, platform.Target{OS: "osx", Arch: "arm64"}, InstallOptions{}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	assertFileExists(t, filepath.Join(targetRoot, "server", "run.sh"))
	assertFileExists(t, filepath.Join(targetRoot, "server", "mesh", "mesh"))
	assertFileExists(t, filepath.Join(targetRoot, "server", "docker", "docker-compose.yaml"))
	assertFileExists(t, filepath.Join(targetRoot, "server", "manifest.json"))
	assertFileExists(t, filepath.Join(targetRoot, "server", "SHA256SUMS.txt"))
	assertFileExists(t, filepath.Join(targetRoot, "engine", "lockstep.engine.dll"))
	assertFileExists(t, filepath.Join(targetRoot, "config", "Luban.dll"))
	assertFileExists(t, filepath.Join(targetRoot, "config", "gen.sh"))
	assertFileExists(t, filepath.Join(targetRoot, "codegen", "lockstep.ecs.generator.dll"))
	assertFileExists(t, filepath.Join(targetRoot, "codegen", "Config", "HashPrimes.json"))
	assertFileExists(t, filepath.Join(targetRoot, "codegen", "scripts", "gen.sh"))

	if err := os.WriteFile(filepath.Join(targetRoot, "server", "stale.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("WriteFile(stale) error = %v", err)
	}
	if err := manager.Install(context.Background(), lockFile, targetRoot, platform.Target{OS: "osx", Arch: "arm64"}, InstallOptions{ForceDownload: true}); err != nil {
		t.Fatalf("forced Install() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetRoot, "server", "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale file still exists after forced reinstall")
	}
}

func TestManagerRejectsMissingCompatibleBundle(t *testing.T) {
	t.Parallel()

	server := testfixtures.NewReleaseServer()
	defer server.Close()

	assets := makeReleaseFixture("server", "v0.2.2",
		manifest.Bundle{
			Name: "server-linux-amd64.zip",
			OS:   "linux",
			Arch: "amd64",
			Validation: manifest.Validation{
				Type:  "bundle-entry-exists",
				Paths: []string{"bin/run.sh"},
			},
		},
		map[string]string{
			"server-linux-amd64.zip": string(testfixtures.BuildZip(map[string]string{
				"bin/run.sh": "#!/usr/bin/env bash\n",
			})),
		},
	)
	server.AddRelease("CDRlease/tgr_server", "v0.2.2", assets)

	client := githubrelease.NewClient(githubrelease.Options{
		BaseURL:    server.BaseURL(),
		HTTPClient: server.HTTPClient(),
	})
	manager := NewManager(client, ioDiscard{})

	lockFile := config.File{
		SchemaVersion: 1,
		Components: map[string]config.Component{
			"server": {Repo: "CDRlease/tgr_server", Tag: "v0.2.2"},
		},
	}

	err := manager.Install(context.Background(), lockFile, t.TempDir(), platform.Target{OS: "osx", Arch: "arm64"}, InstallOptions{})
	if err == nil || !strings.Contains(err.Error(), "no compatible bundle found") {
		t.Fatalf("Install() error = %v, want missing compatible bundle", err)
	}
}

func TestManagerInstallsLatestReleaseAndLogsResolvedTag(t *testing.T) {
	t.Parallel()

	server := testfixtures.NewReleaseServer()
	defer server.Close()

	server.AddRelease("CDRlease/tgr_server", "v0.2.3", makeReleaseFixture("server", "v0.2.3",
		manifest.Bundle{
			Name: "server-osx-arm64.zip",
			OS:   "osx",
			Arch: "arm64",
			Validation: manifest.Validation{
				Type: "bundle-entry-exists",
				Paths: []string{
					"bin/run.sh",
					"bin/mesh/mesh",
				},
			},
		},
		map[string]string{
			"server-osx-arm64.zip": string(testfixtures.BuildZip(map[string]string{
				"bin/run.sh":    "#!/usr/bin/env bash\n",
				"bin/mesh/mesh": "mesh binary",
			})),
		},
	))
	server.SetLatest("CDRlease/tgr_server", "v0.2.3")

	client := githubrelease.NewClient(githubrelease.Options{
		BaseURL:    server.BaseURL(),
		HTTPClient: server.HTTPClient(),
	})

	lockFile := config.File{
		SchemaVersion: 1,
		Components: map[string]config.Component{
			"server": {Repo: "CDRlease/tgr_server", Tag: config.LatestTag},
		},
	}

	targetRoot := t.TempDir()
	var log bytes.Buffer
	manager := NewManager(client, &log)

	if err := manager.Install(context.Background(), lockFile, targetRoot, platform.Target{OS: "osx", Arch: "arm64"}, InstallOptions{}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	assertFileExists(t, filepath.Join(targetRoot, "server", "run.sh"))
	assertFileExists(t, filepath.Join(targetRoot, "server", "mesh", "mesh"))

	output := log.String()
	if !strings.Contains(output, "version        : latest") {
		t.Fatalf("log = %q, want latest version line", output)
	}
	if !strings.Contains(output, "resolved tag   : v0.2.3") {
		t.Fatalf("log = %q, want resolved tag line", output)
	}
}

func TestManagerSkipsDownloadWhenInstalledTagMatches(t *testing.T) {
	t.Parallel()

	server := testfixtures.NewReleaseServer()
	defer server.Close()

	const repo = "CDRlease/tgr_server"
	server.AddRelease(repo, "v0.2.2", makeReleaseFixture("server", "v0.2.2",
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

	client := githubrelease.NewClient(githubrelease.Options{
		BaseURL:    server.BaseURL(),
		HTTPClient: server.HTTPClient(),
	})
	lockFile := config.File{
		SchemaVersion: 1,
		Components: map[string]config.Component{
			"server": {Repo: repo, Tag: "v0.2.2"},
		},
	}

	targetRoot := t.TempDir()
	manager := NewManager(client, ioDiscard{})
	if err := manager.Install(context.Background(), lockFile, targetRoot, platform.Target{OS: "osx", Arch: "arm64"}, InstallOptions{}); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}

	releaseRequests := server.ReleaseRequestCount(repo, "v0.2.2")
	manifestDownloads := server.AssetRequestCount(repo, "manifest.json")
	checksumDownloads := server.AssetRequestCount(repo, "SHA256SUMS.txt")
	bundleDownloads := server.AssetRequestCount(repo, "server-osx-arm64.zip")

	stalePath := filepath.Join(targetRoot, "server", "stale.txt")
	if err := os.WriteFile(stalePath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("WriteFile(stale) error = %v", err)
	}

	var log bytes.Buffer
	manager = NewManager(client, &log)
	if err := manager.Install(context.Background(), lockFile, targetRoot, platform.Target{OS: "osx", Arch: "arm64"}, InstallOptions{}); err != nil {
		t.Fatalf("second Install() error = %v", err)
	}

	assertFileExists(t, stalePath)
	if server.ReleaseRequestCount(repo, "v0.2.2") != releaseRequests {
		t.Fatalf("release requests changed on cache hit")
	}
	if server.AssetRequestCount(repo, "manifest.json") != manifestDownloads {
		t.Fatalf("manifest downloads changed on cache hit")
	}
	if server.AssetRequestCount(repo, "SHA256SUMS.txt") != checksumDownloads {
		t.Fatalf("checksum downloads changed on cache hit")
	}
	if server.AssetRequestCount(repo, "server-osx-arm64.zip") != bundleDownloads {
		t.Fatalf("bundle downloads changed on cache hit")
	}

	output := log.String()
	if !strings.Contains(output, "cache          : hit") || !strings.Contains(output, "download       : skipped") {
		t.Fatalf("log = %q, want cache-hit skip output", output)
	}
}

func TestManagerSkipsLatestDownloadWhenResolvedTagMatches(t *testing.T) {
	t.Parallel()

	server := testfixtures.NewReleaseServer()
	defer server.Close()

	const repo = "CDRlease/tgr_server"
	server.AddRelease(repo, "v0.2.3", makeReleaseFixture("server", "v0.2.3",
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
	server.SetLatest(repo, "v0.2.3")

	client := githubrelease.NewClient(githubrelease.Options{
		BaseURL:    server.BaseURL(),
		HTTPClient: server.HTTPClient(),
	})
	lockFile := config.File{
		SchemaVersion: 1,
		Components: map[string]config.Component{
			"server": {Repo: repo, Tag: config.LatestTag},
		},
	}

	targetRoot := t.TempDir()
	manager := NewManager(client, ioDiscard{})
	if err := manager.Install(context.Background(), lockFile, targetRoot, platform.Target{OS: "osx", Arch: "arm64"}, InstallOptions{}); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}

	latestRequests := server.LatestRequestCount(repo)
	manifestDownloads := server.AssetRequestCount(repo, "manifest.json")
	checksumDownloads := server.AssetRequestCount(repo, "SHA256SUMS.txt")
	bundleDownloads := server.AssetRequestCount(repo, "server-osx-arm64.zip")

	var log bytes.Buffer
	manager = NewManager(client, &log)
	if err := manager.Install(context.Background(), lockFile, targetRoot, platform.Target{OS: "osx", Arch: "arm64"}, InstallOptions{}); err != nil {
		t.Fatalf("second Install() error = %v", err)
	}

	if server.LatestRequestCount(repo) != latestRequests+1 {
		t.Fatalf("latest request count = %d, want %d", server.LatestRequestCount(repo), latestRequests+1)
	}
	if server.AssetRequestCount(repo, "manifest.json") != manifestDownloads {
		t.Fatalf("manifest downloads changed on latest cache hit")
	}
	if server.AssetRequestCount(repo, "SHA256SUMS.txt") != checksumDownloads {
		t.Fatalf("checksum downloads changed on latest cache hit")
	}
	if server.AssetRequestCount(repo, "server-osx-arm64.zip") != bundleDownloads {
		t.Fatalf("bundle downloads changed on latest cache hit")
	}

	output := log.String()
	if !strings.Contains(output, "resolved tag   : v0.2.3") || !strings.Contains(output, "cache          : hit") {
		t.Fatalf("log = %q, want resolved-tag cache-hit output", output)
	}
}

func TestManagerForceDownloadReinstallsMatchingVersion(t *testing.T) {
	t.Parallel()

	server := testfixtures.NewReleaseServer()
	defer server.Close()

	const repo = "CDRlease/tgr_server"
	server.AddRelease(repo, "v0.2.2", makeReleaseFixture("server", "v0.2.2",
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

	client := githubrelease.NewClient(githubrelease.Options{
		BaseURL:    server.BaseURL(),
		HTTPClient: server.HTTPClient(),
	})
	lockFile := config.File{
		SchemaVersion: 1,
		Components: map[string]config.Component{
			"server": {Repo: repo, Tag: "v0.2.2"},
		},
	}

	targetRoot := t.TempDir()
	manager := NewManager(client, ioDiscard{})
	if err := manager.Install(context.Background(), lockFile, targetRoot, platform.Target{OS: "osx", Arch: "arm64"}, InstallOptions{}); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}

	manifestDownloads := server.AssetRequestCount(repo, "manifest.json")
	checksumDownloads := server.AssetRequestCount(repo, "SHA256SUMS.txt")
	bundleDownloads := server.AssetRequestCount(repo, "server-osx-arm64.zip")

	stalePath := filepath.Join(targetRoot, "server", "stale.txt")
	if err := os.WriteFile(stalePath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("WriteFile(stale) error = %v", err)
	}

	var log bytes.Buffer
	manager = NewManager(client, &log)
	if err := manager.Install(context.Background(), lockFile, targetRoot, platform.Target{OS: "osx", Arch: "arm64"}, InstallOptions{ForceDownload: true}); err != nil {
		t.Fatalf("forced Install() error = %v", err)
	}

	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("stale file still exists after forced reinstall")
	}
	if server.AssetRequestCount(repo, "manifest.json") != manifestDownloads+1 {
		t.Fatalf("manifest downloads = %d, want %d", server.AssetRequestCount(repo, "manifest.json"), manifestDownloads+1)
	}
	if server.AssetRequestCount(repo, "SHA256SUMS.txt") != checksumDownloads+1 {
		t.Fatalf("checksum downloads = %d, want %d", server.AssetRequestCount(repo, "SHA256SUMS.txt"), checksumDownloads+1)
	}
	if server.AssetRequestCount(repo, "server-osx-arm64.zip") != bundleDownloads+1 {
		t.Fatalf("bundle downloads = %d, want %d", server.AssetRequestCount(repo, "server-osx-arm64.zip"), bundleDownloads+1)
	}

	output := log.String()
	if !strings.Contains(output, "cache          : bypass (--force-download)") || !strings.Contains(output, "download       : ok") {
		t.Fatalf("log = %q, want forced download output", output)
	}
}

func TestManagerRedownloadsWhenInstalledComponentIsIncomplete(t *testing.T) {
	t.Parallel()

	server := testfixtures.NewReleaseServer()
	defer server.Close()

	const repo = "CDRlease/tgr_server"
	server.AddRelease(repo, "v0.2.2", makeReleaseFixture("server", "v0.2.2",
		manifest.Bundle{
			Name: "server-osx-arm64.zip",
			OS:   "osx",
			Arch: "arm64",
			Validation: manifest.Validation{
				Type: "bundle-entry-exists",
				Paths: []string{
					"bin/run.sh",
					"bin/mesh/mesh",
				},
			},
		},
		map[string]string{
			"server-osx-arm64.zip": string(testfixtures.BuildZip(map[string]string{
				"bin/run.sh":    "#!/usr/bin/env bash\n",
				"bin/mesh/mesh": "mesh binary",
			})),
		},
	))

	client := githubrelease.NewClient(githubrelease.Options{
		BaseURL:    server.BaseURL(),
		HTTPClient: server.HTTPClient(),
	})
	lockFile := config.File{
		SchemaVersion: 1,
		Components: map[string]config.Component{
			"server": {Repo: repo, Tag: "v0.2.2"},
		},
	}

	targetRoot := t.TempDir()
	manager := NewManager(client, ioDiscard{})
	if err := manager.Install(context.Background(), lockFile, targetRoot, platform.Target{OS: "osx", Arch: "arm64"}, InstallOptions{}); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}

	bundleDownloads := server.AssetRequestCount(repo, "server-osx-arm64.zip")
	missingPath := filepath.Join(targetRoot, "server", "mesh", "mesh")
	if err := os.Remove(missingPath); err != nil {
		t.Fatalf("Remove(mesh) error = %v", err)
	}

	var log bytes.Buffer
	manager = NewManager(client, &log)
	if err := manager.Install(context.Background(), lockFile, targetRoot, platform.Target{OS: "osx", Arch: "arm64"}, InstallOptions{}); err != nil {
		t.Fatalf("second Install() error = %v", err)
	}

	assertFileExists(t, missingPath)
	if server.AssetRequestCount(repo, "server-osx-arm64.zip") != bundleDownloads+1 {
		t.Fatalf("bundle downloads = %d, want %d", server.AssetRequestCount(repo, "server-osx-arm64.zip"), bundleDownloads+1)
	}
	output := log.String()
	if !strings.Contains(output, "cache          : miss") || !strings.Contains(output, "download       : ok") {
		t.Fatalf("log = %q, want cache-miss redownload output", output)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
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
