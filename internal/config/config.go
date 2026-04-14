package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

const schemaVersionV1 = 1

type File struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Components    map[string]Component `json:"components"`
}

type Component struct {
	Repo string `json:"repo"`
	Tag  string `json:"tag"`
}

type ResolvedComponent struct {
	Name string
	Repo string
	Tag  string
}

func LoadFile(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("read packages file: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) (File, error) {
	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return File{}, fmt.Errorf("parse packages.json: %w", err)
	}
	if err := file.Validate(); err != nil {
		return File{}, err
	}
	return file, nil
}

func (f File) Validate() error {
	if f.SchemaVersion != schemaVersionV1 {
		return fmt.Errorf("schemaVersion must be 1")
	}

	for name, component := range f.Components {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("component name must not be empty")
		}
		if err := validateRepo(component.Repo); err != nil {
			return fmt.Errorf("component %s: %w", name, err)
		}
		if strings.TrimSpace(component.Tag) == "" {
			return fmt.Errorf("component %s: tag must not be empty", name)
		}
	}

	return nil
}

func NewFile() File {
	return File{
		SchemaVersion: schemaVersionV1,
		Components:    map[string]Component{},
	}
}

func (f File) SortedComponents() []ResolvedComponent {
	names := make([]string, 0, len(f.Components))
	for name := range f.Components {
		names = append(names, name)
	}
	sort.Strings(names)

	components := make([]ResolvedComponent, 0, len(names))
	for _, name := range names {
		component := f.Components[name]
		components = append(components, ResolvedComponent{
			Name: name,
			Repo: component.Repo,
			Tag:  component.Tag,
		})
	}

	return components
}

func validateRepo(repo string) error {
	owner, name, ok := strings.Cut(strings.TrimSpace(repo), "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return fmt.Errorf("repo must be in owner/name format")
	}
	return nil
}
