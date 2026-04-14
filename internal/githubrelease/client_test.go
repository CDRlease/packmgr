package githubrelease

import (
	"net/http"
	"testing"
	"time"
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
