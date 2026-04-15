package githubrelease

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/CDRlease/packmgr/internal/testfixtures"
)

func TestNewClientUsesStreamingFriendlyHTTPClientByDefault(t *testing.T) {
	client := NewClient(Options{})

	if client.httpClient.Timeout != 0 {
		t.Fatalf("default http client timeout = %s, want 0 for large asset streaming", client.httpClient.Timeout)
	}

	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("default transport type = %T, want *http.Transport", client.httpClient.Transport)
	}
	if transport.DialContext == nil {
		t.Fatal("default transport DialContext is nil")
	}
	if transport.ResponseHeaderTimeout != 30*time.Second {
		t.Fatalf("ResponseHeaderTimeout = %s, want %s", transport.ResponseHeaderTimeout, 30*time.Second)
	}
	if transport.TLSHandshakeTimeout != 10*time.Second {
		t.Fatalf("TLSHandshakeTimeout = %s, want %s", transport.TLSHandshakeTimeout, 10*time.Second)
	}
}

func TestNewClientUsesProvidedHTTPClient(t *testing.T) {
	custom := &http.Client{Timeout: 5 * time.Second}

	client := NewClient(Options{HTTPClient: custom})

	if client.httpClient != custom {
		t.Fatal("NewClient did not reuse provided HTTP client")
	}
}

func TestFetchLatestRelease(t *testing.T) {
	t.Parallel()

	server := testfixtures.NewReleaseServer()
	defer server.Close()

	server.AddRelease("CDRlease/tgr_server", "v0.2.3", []testfixtures.AssetSpec{
		{Name: "manifest.json", Content: []byte("{}")},
	})
	server.SetLatest("CDRlease/tgr_server", "v0.2.3")

	client := NewClient(Options{
		BaseURL:    server.BaseURL(),
		HTTPClient: server.HTTPClient(),
	})

	release, err := client.FetchLatestRelease(context.Background(), "CDRlease/tgr_server")
	if err != nil {
		t.Fatalf("FetchLatestRelease() error = %v", err)
	}
	if release.TagName != "v0.2.3" {
		t.Fatalf("release.TagName = %q, want %q", release.TagName, "v0.2.3")
	}
	if _, ok := release.FindAsset("manifest.json"); !ok {
		t.Fatal("FetchLatestRelease() did not include manifest.json asset")
	}
}

func TestFetchLatestReleaseReturnsUsefulErrorWhenMissing(t *testing.T) {
	t.Parallel()

	server := testfixtures.NewReleaseServer()
	defer server.Close()

	client := NewClient(Options{
		BaseURL:    server.BaseURL(),
		HTTPClient: server.HTTPClient(),
	})

	_, err := client.FetchLatestRelease(context.Background(), "CDRlease/tgr_server")
	if err == nil {
		t.Fatal("FetchLatestRelease() error = nil, want missing latest release error")
	}
	if !strings.Contains(err.Error(), "fetch latest release CDRlease/tgr_server") {
		t.Fatalf("FetchLatestRelease() error = %q, want latest release context", err.Error())
	}
}
