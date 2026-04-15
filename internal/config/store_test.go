package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatProducesNormalizedJSON(t *testing.T) {
	t.Parallel()

	data, err := Format(File{
		SchemaVersion: 1,
		Components: map[string]Component{
			"server": {Repo: "CDRlease/tgr_server", Tag: "v0.2.2"},
			"engine": {Repo: "CDRlease/tgr_engine", Tag: "v0.1.1"},
		},
	})
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	const want = `{
  "schemaVersion": 1,
  "components": {
    "engine": {
      "repo": "CDRlease/tgr_engine",
      "tag": "v0.1.1"
    },
    "server": {
      "repo": "CDRlease/tgr_server",
      "tag": "v0.2.2"
    }
  }
}
`
	if string(data) != want {
		t.Fatalf("Format() = %q, want %q", string(data), want)
	}
}

func TestSaveFileRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "packages.json")
	file := NewFile()
	if err := file.AddComponent("server", Component{Repo: "CDRlease/tgr_server", Tag: "v0.2.2"}); err != nil {
		t.Fatalf("AddComponent() error = %v", err)
	}

	if err := SaveFile(path, file); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if loaded.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want 1", loaded.SchemaVersion)
	}
	component, ok := loaded.GetComponent("server")
	if !ok {
		t.Fatalf("GetComponent(server) ok = false, want true")
	}
	if component.Repo != "CDRlease/tgr_server" || component.Tag != "v0.2.2" {
		t.Fatalf("component = %#v, want repo/tag preserved", component)
	}
}

func TestSaveFileRoundTripPreservesLatestTag(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "packages.json")
	file := NewFile()
	if err := file.AddComponent("server", Component{Repo: "CDRlease/tgr_server", Tag: LatestTag}); err != nil {
		t.Fatalf("AddComponent() error = %v", err)
	}

	if err := SaveFile(path, file); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	component, ok := loaded.GetComponent("server")
	if !ok {
		t.Fatalf("GetComponent(server) ok = false, want true")
	}
	if component.Tag != LatestTag {
		t.Fatalf("component.Tag = %q, want %q", component.Tag, LatestTag)
	}
}

func TestSaveFilePreservesExistingPermissions(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "packages.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"components":{}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := SaveFile(path, NewFile()); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("Mode().Perm() = %v, want %v", got, os.FileMode(0o600))
	}
}

func TestAddComponentRejectsDuplicate(t *testing.T) {
	t.Parallel()

	file := NewFile()
	if err := file.AddComponent("server", Component{Repo: "CDRlease/tgr_server", Tag: "v0.2.2"}); err != nil {
		t.Fatalf("AddComponent(first) error = %v", err)
	}
	if err := file.AddComponent("server", Component{Repo: "CDRlease/tgr_server", Tag: "v0.2.3"}); err == nil {
		t.Fatalf("AddComponent(duplicate) error = nil, want duplicate rejection")
	}
}

func TestAddComponentRejectsInvalidAndRollsBack(t *testing.T) {
	t.Parallel()

	file := NewFile()
	err := file.AddComponent("server", Component{Repo: "broken", Tag: "v0.2.2"})
	if err == nil {
		t.Fatalf("AddComponent() error = nil, want validation failure")
	}
	if len(file.Components) != 0 {
		t.Fatalf("len(file.Components) = %d, want 0 after rollback", len(file.Components))
	}
}

func TestUpdateComponent(t *testing.T) {
	t.Parallel()

	file := NewFile()
	if err := file.AddComponent("server", Component{Repo: "CDRlease/tgr_server", Tag: "v0.2.2"}); err != nil {
		t.Fatalf("AddComponent() error = %v", err)
	}

	repo := "CDRlease/tgr_server_new"
	tag := "v0.2.3"
	changed, err := file.UpdateComponent("server", ComponentPatch{Repo: &repo, Tag: &tag})
	if err != nil {
		t.Fatalf("UpdateComponent() error = %v", err)
	}
	if !changed {
		t.Fatalf("UpdateComponent() changed = false, want true")
	}

	component, _ := file.GetComponent("server")
	if component.Repo != repo || component.Tag != tag {
		t.Fatalf("component = %#v, want updated repo/tag", component)
	}
}

func TestUpdateComponentNoop(t *testing.T) {
	t.Parallel()

	file := NewFile()
	if err := file.AddComponent("server", Component{Repo: "CDRlease/tgr_server", Tag: "v0.2.2"}); err != nil {
		t.Fatalf("AddComponent() error = %v", err)
	}

	repo := "CDRlease/tgr_server"
	changed, err := file.UpdateComponent("server", ComponentPatch{Repo: &repo})
	if err != nil {
		t.Fatalf("UpdateComponent() error = %v", err)
	}
	if changed {
		t.Fatalf("UpdateComponent() changed = true, want false")
	}
}

func TestUpdateComponentRejectsInvalidAndRollsBack(t *testing.T) {
	t.Parallel()

	file := NewFile()
	if err := file.AddComponent("server", Component{Repo: "CDRlease/tgr_server", Tag: "v0.2.2"}); err != nil {
		t.Fatalf("AddComponent() error = %v", err)
	}

	repo := "broken"
	changed, err := file.UpdateComponent("server", ComponentPatch{Repo: &repo})
	if err == nil {
		t.Fatalf("UpdateComponent() error = nil, want validation failure")
	}
	if changed {
		t.Fatalf("UpdateComponent() changed = true, want false")
	}

	component, _ := file.GetComponent("server")
	if component.Repo != "CDRlease/tgr_server" || component.Tag != "v0.2.2" {
		t.Fatalf("component = %#v, want rollback to original value", component)
	}
}

func TestUpdateComponentSupportsLatestTag(t *testing.T) {
	t.Parallel()

	file := NewFile()
	if err := file.AddComponent("server", Component{Repo: "CDRlease/tgr_server", Tag: "v0.2.2"}); err != nil {
		t.Fatalf("AddComponent() error = %v", err)
	}

	tag := LatestTag
	changed, err := file.UpdateComponent("server", ComponentPatch{Tag: &tag})
	if err != nil {
		t.Fatalf("UpdateComponent() error = %v", err)
	}
	if !changed {
		t.Fatalf("UpdateComponent() changed = false, want true")
	}

	component, _ := file.GetComponent("server")
	if component.Tag != LatestTag {
		t.Fatalf("component.Tag = %q, want %q", component.Tag, LatestTag)
	}
}

func TestRemoveComponent(t *testing.T) {
	t.Parallel()

	file := NewFile()
	if err := file.AddComponent("server", Component{Repo: "CDRlease/tgr_server", Tag: "v0.2.2"}); err != nil {
		t.Fatalf("AddComponent() error = %v", err)
	}

	if err := file.RemoveComponent("server"); err != nil {
		t.Fatalf("RemoveComponent() error = %v", err)
	}
	if _, ok := file.GetComponent("server"); ok {
		t.Fatalf("GetComponent(server) ok = true, want false")
	}
}

func TestRemoveComponentRejectsMissing(t *testing.T) {
	t.Parallel()

	file := NewFile()
	if err := file.RemoveComponent("server"); err == nil {
		t.Fatalf("RemoveComponent() error = nil, want missing component rejection")
	}
}

func TestSaveFileRejectsInvalidAndLeavesExistingContentUntouched(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "packages.json")
	original := []byte("{\n  \"schemaVersion\": 1,\n  \"components\": {}\n}\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err := SaveFile(path, File{
		SchemaVersion: 1,
		Components: map[string]Component{
			"server": {Repo: "broken", Tag: "v0.2.2"},
		},
	})
	if err == nil {
		t.Fatalf("SaveFile() error = nil, want validation failure")
	}

	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(got) != string(original) {
		t.Fatalf("file contents changed after failed SaveFile()")
	}
}
