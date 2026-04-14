package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ComponentPatch struct {
	Repo *string
	Tag  *string
}

func (f File) GetComponent(name string) (Component, bool) {
	component, ok := f.Components[name]
	return component, ok
}

func (f *File) AddComponent(name string, component Component) error {
	if f.Components == nil {
		f.Components = map[string]Component{}
	}
	if _, exists := f.Components[name]; exists {
		return fmt.Errorf("component %s already exists", name)
	}

	f.Components[name] = component
	if err := f.Validate(); err != nil {
		delete(f.Components, name)
		return err
	}

	return nil
}

func (f *File) UpdateComponent(name string, patch ComponentPatch) (bool, error) {
	component, ok := f.Components[name]
	if !ok {
		return false, fmt.Errorf("component %s not found", name)
	}

	updated := component
	if patch.Repo != nil {
		updated.Repo = *patch.Repo
	}
	if patch.Tag != nil {
		updated.Tag = *patch.Tag
	}

	if updated == component {
		return false, nil
	}

	f.Components[name] = updated
	if err := f.Validate(); err != nil {
		f.Components[name] = component
		return false, err
	}

	return true, nil
}

func (f *File) RemoveComponent(name string) error {
	if _, ok := f.Components[name]; !ok {
		return fmt.Errorf("component %s not found", name)
	}

	delete(f.Components, name)
	return f.Validate()
}

func Format(file File) ([]byte, error) {
	normalized := normalize(file)
	if err := normalized.Validate(); err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal packages.json: %w", err)
	}

	return append(data, '\n'), nil
}

func SaveFile(path string, file File) error {
	data, err := Format(file)
	if err != nil {
		return err
	}

	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat packages file: %w", err)
	}

	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, ".packmgr-packages-*.json")
	if err != nil {
		return fmt.Errorf("create temp packages file: %w", err)
	}

	tempPath := tempFile.Name()
	cleanup := func() {
		tempFile.Close()
		_ = os.Remove(tempPath)
	}

	if _, err := tempFile.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp packages file: %w", err)
	}
	if err := tempFile.Chmod(mode); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp packages file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close temp packages file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace packages file: %w", err)
	}

	return nil
}

func normalize(file File) File {
	normalized := file
	if normalized.SchemaVersion == 0 {
		normalized.SchemaVersion = schemaVersionV1
	}
	if normalized.Components == nil {
		normalized.Components = map[string]Component{}
	}
	return normalized
}
