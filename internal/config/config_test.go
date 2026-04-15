package config

import "testing"

func TestParseValidFile(t *testing.T) {
	t.Parallel()

	file, err := Parse([]byte(`{
		"schemaVersion": 1,
		"components": {
			"server": {
				"repo": "CDRlease/tgr_server",
				"tag": "v0.2.2"
			}
		}
	}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if got := file.Components["server"].Repo; got != "CDRlease/tgr_server" {
		t.Fatalf("Repo = %q, want %q", got, "CDRlease/tgr_server")
	}
}

func TestParseRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		data string
	}{
		{
			name: "invalid schema",
			data: `{"schemaVersion":2,"components":{"server":{"repo":"CDRlease/tgr_server","tag":"v0.2.2"}}}`,
		},
		{
			name: "invalid repo",
			data: `{"schemaVersion":1,"components":{"server":{"repo":"broken","tag":"v0.2.2"}}}`,
		},
		{
			name: "empty tag",
			data: `{"schemaVersion":1,"components":{"server":{"repo":"CDRlease/tgr_server","tag":""}}}`,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Parse([]byte(testCase.data)); err == nil {
				t.Fatalf("Parse() error = nil, want error")
			}
		})
	}
}

func TestParseAllowsEmptyComponents(t *testing.T) {
	t.Parallel()

	file, err := Parse([]byte(`{"schemaVersion":1,"components":{}}`))
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if len(file.Components) != 0 {
		t.Fatalf("len(file.Components) = %d, want 0", len(file.Components))
	}
}

func TestParseAllowsLatestTag(t *testing.T) {
	t.Parallel()

	file, err := Parse([]byte(`{
		"schemaVersion": 1,
		"components": {
			"server": {
				"repo": "CDRlease/tgr_server",
				"tag": "latest"
			}
		}
	}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	got := file.Components["server"].Tag
	if got != LatestTag {
		t.Fatalf("Tag = %q, want %q", got, LatestTag)
	}
	if !IsLatestTag(got) {
		t.Fatalf("IsLatestTag(%q) = false, want true", got)
	}
}

func TestSortedComponents(t *testing.T) {
	t.Parallel()

	file := File{
		SchemaVersion: 1,
		Components: map[string]Component{
			"engine": {Repo: "CDRlease/tgr_engine", Tag: "v0.1.1"},
			"server": {Repo: "CDRlease/tgr_server", Tag: "v0.2.2"},
		},
	}

	components := file.SortedComponents()
	if len(components) != 2 {
		t.Fatalf("len(components) = %d, want 2", len(components))
	}
	if components[0].Name != "engine" || components[1].Name != "server" {
		t.Fatalf("SortedComponents() = %#v, want alphabetical order", components)
	}
}
