package platform

import (
	"fmt"
	"runtime"
)

type Target struct {
	OS   string
	Arch string
}

func Detect() (Target, error) {
	return Normalize(runtime.GOOS, runtime.GOARCH)
}

func Normalize(goos, goarch string) (Target, error) {
	target := Target{}

	switch goos {
	case "darwin":
		target.OS = "osx"
	case "linux":
		target.OS = "linux"
	case "windows":
		target.OS = "win"
	default:
		return Target{}, fmt.Errorf("unsupported OS: %s", goos)
	}

	switch goarch {
	case "amd64":
		target.Arch = "amd64"
	case "arm64":
		target.Arch = "arm64"
	default:
		return Target{}, fmt.Errorf("unsupported architecture: %s", goarch)
	}

	return target, nil
}
