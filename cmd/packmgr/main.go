package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/CDRlease/packmgr/internal/config"
	"github.com/CDRlease/packmgr/internal/githubrelease"
	"github.com/CDRlease/packmgr/internal/install"
	"github.com/CDRlease/packmgr/internal/platform"
)

var Version = "dev"

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
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "version":
		fmt.Fprintf(stdout, "%s\n", Version)
		return 0
	case "help", "--help", "-h":
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

	packagesPath := fs.String("packages", "", "Path to packages.json")
	targetDir := fs.String("dir", "", "Installation target directory")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *packagesPath == "" {
		return fmt.Errorf("--packages is required")
	}
	if *targetDir == "" {
		return fmt.Errorf("--dir is required")
	}

	lockFile, err := config.LoadFile(*packagesPath)
	if err != nil {
		return err
	}

	target, err := platform.Detect()
	if err != nil {
		return err
	}

	client := githubrelease.NewClient(githubrelease.Options{
		Token: githubrelease.TokenFromEnv(),
	})
	manager := install.NewManager(client, stdout)

	fmt.Fprintf(stdout, "packmgr %s\n", Version)
	fmt.Fprintf(stdout, "packages file : %s\n", *packagesPath)
	fmt.Fprintf(stdout, "target dir    : %s\n", *targetDir)
	fmt.Fprintf(stdout, "detected os   : %s\n", target.OS)
	fmt.Fprintf(stdout, "detected arch : %s\n\n", target.Arch)

	return manager.Install(context.Background(), lockFile, *targetDir, target)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  packmgr install --packages <packages.json> --dir <target-dir>")
	fmt.Fprintln(w, "  packmgr version")
}
