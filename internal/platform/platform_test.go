package platform

import "testing"

func TestNormalize(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		goos   string
		goarch string
		want   Target
	}{
		{name: "darwin arm64", goos: "darwin", goarch: "arm64", want: Target{OS: "osx", Arch: "arm64"}},
		{name: "linux amd64", goos: "linux", goarch: "amd64", want: Target{OS: "linux", Arch: "amd64"}},
		{name: "windows amd64", goos: "windows", goarch: "amd64", want: Target{OS: "win", Arch: "amd64"}},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := Normalize(testCase.goos, testCase.goarch)
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}
			if got != testCase.want {
				t.Fatalf("Normalize() = %#v, want %#v", got, testCase.want)
			}
		})
	}
}

func TestNormalizeRejectsUnsupportedValues(t *testing.T) {
	t.Parallel()

	if _, err := Normalize("plan9", "amd64"); err == nil {
		t.Fatalf("Normalize() error = nil, want unsupported OS error")
	}
	if _, err := Normalize("linux", "386"); err == nil {
		t.Fatalf("Normalize() error = nil, want unsupported arch error")
	}
}
