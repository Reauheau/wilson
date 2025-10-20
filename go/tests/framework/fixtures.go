package framework

import (
	"fmt"
	"os"
	"path/filepath"
)

// FixtureManager handles test fixtures
type FixtureManager struct {
	baseDir string
}

// NewFixtureManager creates a new fixture manager
func NewFixtureManager(baseDir string) *FixtureManager {
	return &FixtureManager{baseDir: baseDir}
}

// Load loads a fixture by name
func (fm *FixtureManager) Load(name string) (string, error) {
	path := filepath.Join(fm.baseDir, name)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to load fixture %s: %w", name, err)
	}
	return string(content), nil
}

// LoadDir loads all files from a fixture directory
func (fm *FixtureManager) LoadDir(dir string) (map[string]string, error) {
	path := filepath.Join(fm.baseDir, dir)
	files := make(map[string]string)

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(path, filePath)
		if err != nil {
			return err
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		files[relPath] = string(content)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load fixture directory %s: %w", dir, err)
	}

	return files, nil
}

// Exists checks if a fixture exists
func (fm *FixtureManager) Exists(name string) bool {
	path := filepath.Join(fm.baseDir, name)
	_, err := os.Stat(path)
	return err == nil
}

// GetPath returns the full path to a fixture
func (fm *FixtureManager) GetPath(name string) string {
	return filepath.Join(fm.baseDir, name)
}
