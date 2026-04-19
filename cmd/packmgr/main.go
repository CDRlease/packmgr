package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/CDRlease/packmgr/internal/config"
	"github.com/CDRlease/packmgr/internal/githubrelease"
	"github.com/CDRlease/packmgr/internal/install"
	"github.com/CDRlease/packmgr/internal/platform"
)

var Version = "dev"

var defaultPackagesPath = "./packages.json"

var detectPlatform = platform.Detect

var newReleaseClient = func() *githubrelease.Client {
	return githubrelease.NewClient(githubrelease.Options{
		Token: githubrelease.TokenFromEnv(),
	})
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "install":
		if err := runInstall(args[1:], stdout); err != nil {
			return printCommandError(stderr, err)
		}
		return 0
	case "packages":
		if err := runPackages(args[1:], stdout); err != nil {
			return printCommandError(stderr, err)
		}
		return 0
	case "version":
		fmt.Fprintf(stdout, "%s\n", Version)
		return 0
	case "help", "--help", "-h":
		if len(args) > 1 {
			switch args[1] {
			case "install":
				printInstallUsage(stdout)
				return 0
			case "packages":
				printPackagesUsage(stdout)
				return 0
			default:
				printUsage(stdout)
				return 0
			}
		}
		printUsage(stdout)
		return 0
	default:
		printUsage(stderr)
		return 2
	}
}

func runInstall(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	packagesPath := fs.String("packages", defaultPackagesPath, "Path to packages.json")
	targetDir := fs.String("dir", "", "Installation target directory")
	forceDownload := fs.Bool("force-download", false, "Always redownload and replace matching installed versions")

	if err := parseFlags(fs, args); err != nil {
		return usageError{err}
	}
	if fs.NArg() != 0 {
		return usageError{fmt.Errorf("install does not accept positional arguments")}
	}
	if *targetDir == "" {
		return usageError{fmt.Errorf("--dir is required")}
	}

	lockFile, err := config.LoadFile(*packagesPath)
	if err != nil {
		return err
	}

	target, err := detectPlatform()
	if err != nil {
		return err
	}

	manager := install.NewManager(newReleaseClient(), stdout)

	fmt.Fprintf(stdout, "packmgr %s\n", Version)
	fmt.Fprintf(stdout, "packages file : %s\n", *packagesPath)
	fmt.Fprintf(stdout, "target dir    : %s\n", *targetDir)
	fmt.Fprintf(stdout, "detected os   : %s\n", target.OS)
	fmt.Fprintf(stdout, "detected arch : %s\n\n", target.Arch)

	return manager.Install(context.Background(), lockFile, *targetDir, target, install.InstallOptions{
		ForceDownload: *forceDownload,
	})
}

func runPackages(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError{fmt.Errorf("packages subcommand is required")}
	}

	switch args[0] {
	case "list":
		return runPackagesList(args[1:], stdout)
	case "get":
		return runPackagesGet(args[1:], stdout)
	case "add":
		return runPackagesAdd(args[1:], stdout)
	case "update":
		return runPackagesUpdate(args[1:], stdout)
	case "remove":
		return runPackagesRemove(args[1:], stdout)
	case "help", "--help", "-h":
		printPackagesUsage(stdout)
		return nil
	default:
		return usageError{fmt.Errorf("unknown packages subcommand: %s", args[0])}
	}
}

func runPackagesList(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("packages list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	packagesPath := fs.String("packages", defaultPackagesPath, "Path to packages.json")
	jsonOutput := fs.Bool("json", false, "Print JSON output")

	if err := parseFlags(fs, args); err != nil {
		return usageError{err}
	}
	if fs.NArg() != 0 {
		return usageError{fmt.Errorf("list does not accept positional arguments")}
	}

	file, err := config.LoadFile(*packagesPath)
	if err != nil {
		return err
	}

	if *jsonOutput {
		return writeFileJSON(stdout, file)
	}

	for _, component := range file.SortedComponents() {
		fmt.Fprintf(stdout, "%s repo=%s tag=%s\n", component.Name, component.Repo, component.Tag)
	}
	return nil
}

func runPackagesGet(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("packages get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	packagesPath := fs.String("packages", defaultPackagesPath, "Path to packages.json")
	jsonOutput := fs.Bool("json", false, "Print JSON output")

	if err := parseFlags(fs, args); err != nil {
		return usageError{err}
	}
	if fs.NArg() != 1 {
		return usageError{fmt.Errorf("get requires exactly 1 component name")}
	}

	file, err := config.LoadFile(*packagesPath)
	if err != nil {
		return err
	}

	name := fs.Arg(0)
	component, ok := file.GetComponent(name)
	if !ok {
		return fmt.Errorf("component %s not found", name)
	}

	if *jsonOutput {
		return writeJSON(stdout, struct {
			Name string `json:"name"`
			Repo string `json:"repo"`
			Tag  string `json:"tag"`
		}{
			Name: name,
			Repo: component.Repo,
			Tag:  component.Tag,
		})
	}

	fmt.Fprintf(stdout, "name: %s\n", name)
	fmt.Fprintf(stdout, "repo: %s\n", component.Repo)
	fmt.Fprintf(stdout, "tag: %s\n", component.Tag)
	return nil
}

func runPackagesAdd(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("packages add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	packagesPath := fs.String("packages", defaultPackagesPath, "Path to packages.json")
	repo := fs.String("repo", "", "Component repository in owner/name format")
	tag := fs.String("tag", "", "Release tag")
	checkRelease := fs.Bool("check-release", false, "Verify the release exists before writing")

	if err := parseFlags(fs, args); err != nil {
		return usageError{err}
	}
	if fs.NArg() != 1 {
		return usageError{fmt.Errorf("add requires exactly 1 component name")}
	}
	if strings.TrimSpace(*repo) == "" {
		return usageError{fmt.Errorf("--repo is required")}
	}
	if strings.TrimSpace(*tag) == "" {
		return usageError{fmt.Errorf("--tag is required")}
	}

	name := fs.Arg(0)
	file, err := loadPackagesForWrite(*packagesPath, true)
	if err != nil {
		return err
	}
	component := config.Component{Repo: *repo, Tag: *tag}
	if err := file.AddComponent(name, component); err != nil {
		return err
	}
	if *checkRelease {
		if err := ensureReleaseExists(component.Repo, component.Tag); err != nil {
			return err
		}
	}
	if err := config.SaveFile(*packagesPath, file); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "added component %s\n", name)
	return nil
}

func runPackagesUpdate(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("packages update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	packagesPath := fs.String("packages", defaultPackagesPath, "Path to packages.json")
	checkRelease := fs.Bool("check-release", false, "Verify the release exists before writing")

	var repo optionalStringFlag
	var tag optionalStringFlag
	fs.Var(&repo, "repo", "Component repository in owner/name format")
	fs.Var(&tag, "tag", "Release tag")

	if err := parseFlags(fs, args); err != nil {
		return usageError{err}
	}
	if fs.NArg() != 1 {
		return usageError{fmt.Errorf("update requires exactly 1 component name")}
	}
	if !repo.set && !tag.set {
		return usageError{fmt.Errorf("at least one of --repo or --tag is required")}
	}

	name := fs.Arg(0)
	file, err := loadPackagesForWrite(*packagesPath, false)
	if err != nil {
		return err
	}

	if _, ok := file.GetComponent(name); !ok {
		return fmt.Errorf("component %s not found", name)
	}

	patch := config.ComponentPatch{}
	if repo.set {
		patch.Repo = &repo.value
	}
	if tag.set {
		patch.Tag = &tag.value
	}

	changed, err := file.UpdateComponent(name, patch)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Fprintf(stdout, "component %s unchanged\n", name)
		return nil
	}
	updated, _ := file.GetComponent(name)
	if *checkRelease {
		if err := ensureReleaseExists(updated.Repo, updated.Tag); err != nil {
			return err
		}
	}
	if err := config.SaveFile(*packagesPath, file); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "updated component %s\n", name)
	return nil
}

func runPackagesRemove(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("packages remove", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	packagesPath := fs.String("packages", defaultPackagesPath, "Path to packages.json")

	if err := parseFlags(fs, args); err != nil {
		return usageError{err}
	}
	if fs.NArg() != 1 {
		return usageError{fmt.Errorf("remove requires exactly 1 component name")}
	}

	name := fs.Arg(0)
	file, err := loadPackagesForWrite(*packagesPath, false)
	if err != nil {
		return err
	}
	if err := file.RemoveComponent(name); err != nil {
		return err
	}
	if err := config.SaveFile(*packagesPath, file); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "removed component %s\n", name)
	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  packmgr install [--packages <packages.json>] --dir <target-dir> [--force-download]")
	fmt.Fprintln(w, "  packmgr packages <subcommand> [flags]")
	fmt.Fprintln(w, "  packmgr version")
	fmt.Fprintln(w, "  packmgr help packages")
	fmt.Fprintln(w, "  packmgr help install")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  install   Download, verify, and install release bundles into a target directory")
	fmt.Fprintln(w, "  packages  Inspect and maintain packages.json entries")
	fmt.Fprintln(w, "  version   Print the current packmgr version")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Defaults:")
	fmt.Fprintf(w, "  packages file: %s\n", defaultPackagesPath)
	fmt.Fprintln(w, "  GitHub token lookup order: PACKMGR_GITHUB_TOKEN, GH_TOKEN, GITHUB_TOKEN")
	fmt.Fprintln(w, "  latest means GitHub's official latest stable release")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  packmgr install --dir ./vendor")
	fmt.Fprintln(w, "  packmgr packages list")
	fmt.Fprintln(w, "  packmgr packages get server --json")
	fmt.Fprintln(w, "  packmgr packages add tools --repo owner/tools --tag latest --check-release")
}

func printInstallUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  packmgr install [--packages <packages.json>] --dir <target-dir> [--force-download]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Description:")
	fmt.Fprintln(w, "  Read packages.json, detect the current OS and architecture, then download,")
	fmt.Fprintln(w, "  verify, and install the matching release bundles into the target directory.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintf(w, "  --packages <path>  Path to packages.json (default %s)\n", defaultPackagesPath)
	fmt.Fprintln(w, "  --dir <path>       Installation target directory (required)")
	fmt.Fprintln(w, "  --force-download   Ignore matching installed versions and redownload assets")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  packmgr install --dir ./vendor")
	fmt.Fprintln(w, "  packmgr install --packages ./examples/packages.json --dir ./vendor")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Install layout:")
	fmt.Fprintln(w, "  Each component is installed as a flattened directory that keeps payload files,")
	fmt.Fprintln(w, "  manifest.json, and SHA256SUMS.txt directly under <target-dir>/<component>/")
	fmt.Fprintln(w, "  Matching installed versions are treated as cache hits and skipped by default")
	fmt.Fprintln(w, "  When tag=latest, packmgr resolves GitHub's official latest stable release at install time")
}

func printPackagesUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  packmgr packages list [--packages <packages.json>] [--json]")
	fmt.Fprintln(w, "  packmgr packages get <name> [--packages <packages.json>] [--json]")
	fmt.Fprintln(w, "  packmgr packages add <name> --repo <owner/name> --tag <tag> [--packages <packages.json>] [--check-release]")
	fmt.Fprintln(w, "  packmgr packages update <name> [--repo <owner/name>] [--tag <tag>] [--packages <packages.json>] [--check-release]")
	fmt.Fprintln(w, "  packmgr packages remove <name> [--packages <packages.json>]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Description:")
	fmt.Fprintln(w, "  Inspect and maintain the components stored in packages.json.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Defaults:")
	fmt.Fprintf(w, "  --packages defaults to %s\n", defaultPackagesPath)
	fmt.Fprintln(w, "  add creates a new packages.json when the target file does not exist")
	fmt.Fprintln(w, "  tag=latest resolves GitHub's official latest stable release during install/check-release")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Outputs:")
	fmt.Fprintln(w, "  list        text: <name> repo=<repo> tag=<tag>")
	fmt.Fprintln(w, "  get         text: name:/repo:/tag: on separate lines")
	fmt.Fprintln(w, "  list --json prints the normalized full packages.json document")
	fmt.Fprintln(w, "  get --json  prints a single component object with name/repo/tag")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  packmgr packages list")
	fmt.Fprintln(w, "  packmgr packages get server --json")
	fmt.Fprintln(w, "  packmgr packages add server --repo CDRlease/tgr_server --tag latest --check-release")
	fmt.Fprintln(w, "  packmgr packages update server --tag latest --check-release")
	fmt.Fprintln(w, "  packmgr packages remove server")
}

func printCommandError(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "error: %v\n", err)
	var usage usageError
	if errors.As(err, &usage) {
		return 2
	}
	return 1
}

func loadPackagesForWrite(path string, allowMissing bool) (config.File, error) {
	file, err := config.LoadFile(path)
	if err == nil {
		return file, nil
	}
	if allowMissing && errors.Is(err, os.ErrNotExist) {
		return config.NewFile(), nil
	}
	return config.File{}, err
}

func ensureReleaseExists(repo, tag string) error {
	client := newReleaseClient()
	if config.IsLatestTag(tag) {
		_, err := client.FetchLatestRelease(context.Background(), repo)
		return err
	}
	_, err := client.FetchRelease(context.Background(), repo, tag)
	return err
}

func writeFileJSON(stdout io.Writer, file config.File) error {
	data, err := config.Format(file)
	if err != nil {
		return err
	}
	_, err = stdout.Write(data)
	return err
}

func writeJSON(stdout io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = stdout.Write(data)
	return err
}

func parseFlags(fs *flag.FlagSet, args []string) error {
	return fs.Parse(reorderInterspersedArgs(fs, args))
}

func reorderInterspersedArgs(fs *flag.FlagSet, args []string) []string {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}

		flags = append(flags, arg)
		if strings.Contains(arg, "=") {
			continue
		}

		name := strings.TrimLeft(arg, "-")
		if flagDef := fs.Lookup(name); flagDef != nil {
			if boolFlag, ok := flagDef.Value.(interface{ IsBoolFlag() bool }); ok && boolFlag.IsBoolFlag() {
				continue
			}
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
		}
	}

	return append(flags, positionals...)
}

type usageError struct {
	err error
}

func (e usageError) Error() string {
	return e.err.Error()
}

func (e usageError) Unwrap() error {
	return e.err
}

type optionalStringFlag struct {
	value string
	set   bool
}

func (f *optionalStringFlag) String() string {
	return f.value
}

func (f *optionalStringFlag) Set(value string) error {
	f.value = value
	f.set = true
	return nil
}
