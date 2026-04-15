package testfixtures

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
)

type ReleaseServer struct {
	server   *httptest.Server
	releases map[string]map[string]*ReleaseFixture
	latest   map[string]string
}

type ReleaseFixture struct {
	Tag    string
	Assets map[string][]byte
}

type AssetSpec struct {
	Name    string
	Content []byte
}

func NewReleaseServer() *ReleaseServer {
	fixture := &ReleaseServer{
		releases: make(map[string]map[string]*ReleaseFixture),
		latest:   make(map[string]string),
	}
	fixture.server = httptest.NewServer(http.HandlerFunc(fixture.handle))
	return fixture
}

func (r *ReleaseServer) Close() {
	r.server.Close()
}

func (r *ReleaseServer) BaseURL() string {
	return r.server.URL
}

func (r *ReleaseServer) HTTPClient() *http.Client {
	return r.server.Client()
}

func (r *ReleaseServer) AddRelease(repo string, tag string, assets []AssetSpec) {
	repoReleases := r.releases[repo]
	if repoReleases == nil {
		repoReleases = make(map[string]*ReleaseFixture)
		r.releases[repo] = repoReleases
	}

	fixture := &ReleaseFixture{
		Tag:    tag,
		Assets: make(map[string][]byte, len(assets)),
	}
	for _, asset := range assets {
		fixture.Assets[asset.Name] = asset.Content
	}
	repoReleases[tag] = fixture
}

func (r *ReleaseServer) SetLatest(repo, tag string) {
	r.latest[repo] = tag
}

func (r *ReleaseServer) handle(w http.ResponseWriter, request *http.Request) {
	if strings.HasPrefix(request.URL.Path, "/repos/") && strings.Contains(request.URL.Path, "/releases/tags/") {
		r.handleRelease(w, request)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/repos/") && strings.HasSuffix(request.URL.Path, "/releases/latest") {
		r.handleLatestRelease(w, request)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/downloads/") {
		r.handleAsset(w, request)
		return
	}
	http.NotFound(w, request)
}

func (r *ReleaseServer) handleRelease(w http.ResponseWriter, request *http.Request) {
	remainder := strings.TrimPrefix(request.URL.Path, "/repos/")
	parts := strings.Split(remainder, "/")
	if len(parts) < 5 {
		http.NotFound(w, request)
		return
	}
	repo := parts[0] + "/" + parts[1]
	tag := parts[len(parts)-1]

	fixture := r.releases[repo][tag]
	if fixture == nil {
		http.NotFound(w, request)
		return
	}

	type assetJSON struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}
	response := struct {
		TagName string      `json:"tag_name"`
		Assets  []assetJSON `json:"assets"`
	}{
		TagName: tag,
		Assets:  make([]assetJSON, 0, len(fixture.Assets)),
	}

	for name := range fixture.Assets {
		response.Assets = append(response.Assets, assetJSON{
			Name:               name,
			BrowserDownloadURL: fmt.Sprintf("%s/downloads/%s/%s/%s", r.server.URL, parts[0], parts[1], path.Clean(name)),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (r *ReleaseServer) handleLatestRelease(w http.ResponseWriter, request *http.Request) {
	remainder := strings.TrimPrefix(request.URL.Path, "/repos/")
	parts := strings.Split(remainder, "/")
	if len(parts) < 4 {
		http.NotFound(w, request)
		return
	}
	repo := parts[0] + "/" + parts[1]
	tag, ok := r.latest[repo]
	if !ok {
		http.NotFound(w, request)
		return
	}

	fixture := r.releases[repo][tag]
	if fixture == nil {
		http.NotFound(w, request)
		return
	}

	type assetJSON struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}
	response := struct {
		TagName string      `json:"tag_name"`
		Assets  []assetJSON `json:"assets"`
	}{
		TagName: tag,
		Assets:  make([]assetJSON, 0, len(fixture.Assets)),
	}

	for name := range fixture.Assets {
		response.Assets = append(response.Assets, assetJSON{
			Name:               name,
			BrowserDownloadURL: fmt.Sprintf("%s/downloads/%s/%s/%s", r.server.URL, parts[0], parts[1], path.Clean(name)),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (r *ReleaseServer) handleAsset(w http.ResponseWriter, request *http.Request) {
	remainder := strings.TrimPrefix(request.URL.Path, "/downloads/")
	parts := strings.SplitN(remainder, "/", 3)
	if len(parts) != 3 {
		http.NotFound(w, request)
		return
	}
	repo := parts[0] + "/" + parts[1]
	assetName := parts[2]

	for _, fixture := range r.releases[repo] {
		if body, ok := fixture.Assets[assetName]; ok {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
			return
		}
	}

	http.NotFound(w, request)
}

func BuildZip(entries map[string]string) []byte {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range entries {
		fileWriter, err := writer.Create(name)
		if err != nil {
			panic(err)
		}
		if _, err := fileWriter.Write([]byte(content)); err != nil {
			panic(err)
		}
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}
	return buffer.Bytes()
}
