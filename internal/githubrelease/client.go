package githubrelease

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultAPIBaseURL = "https://api.github.com"

type Options struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name            string `json:"name"`
	BrowserDownload string `json:"browser_download_url"`
}

func NewClient(opts Options) *Client {
	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultAPIBaseURL
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	return &Client{
		baseURL:    baseURL,
		token:      opts.Token,
		httpClient: httpClient,
	}
}

func TokenFromEnv() string {
	for _, key := range []string{"PACKMGR_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func (c *Client) FetchRelease(ctx context.Context, repo, tag string) (*Release, error) {
	requestURL := fmt.Sprintf("%s/repos/%s/releases/tags/%s", c.baseURL, repo, url.PathEscape(tag))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	c.decorateAPIRequest(request)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch release %s@%s: %w", repo, tag, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		return nil, fmt.Errorf("fetch release %s@%s: unexpected status %d: %s", repo, tag, response.StatusCode, strings.TrimSpace(string(body)))
	}

	var release Release
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release %s@%s: %w", repo, tag, err)
	}
	return &release, nil
}

func (c *Client) DownloadAsset(ctx context.Context, asset Asset, destination string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownload, nil)
	if err != nil {
		return err
	}
	c.decorateDownloadRequest(request)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("download asset %s: %w", asset.Name, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		return fmt.Errorf("download asset %s: unexpected status %d: %s", asset.Name, response.StatusCode, strings.TrimSpace(string(body)))
	}

	file, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("create %s: %w", destination, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, response.Body); err != nil {
		return fmt.Errorf("write %s: %w", destination, err)
	}
	return nil
}

func (r Release) FindAsset(name string) (Asset, bool) {
	for _, asset := range r.Assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return Asset{}, false
}

func (c *Client) decorateAPIRequest(request *http.Request) {
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "packmgr")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func (c *Client) decorateDownloadRequest(request *http.Request) {
	request.Header.Set("User-Agent", "packmgr")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
}
