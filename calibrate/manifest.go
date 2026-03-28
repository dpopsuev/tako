package calibrate

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	yaml "go.yaml.in/yaml/v3"
)

// WriteManifest serialises a Manifest to manifest.yaml inside dir.
func WriteManifest(dir string, m *Manifest) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, ManifestFile), data, 0o644)
}

// ReadManifest parses manifest.yaml from an fs.FS bundle root.
func ReadManifest(fsys fs.FS) (*Manifest, error) {
	data, err := fs.ReadFile(fsys, ManifestFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ManifestFile, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", ManifestFile, err)
	}
	return &m, nil
}

// ValidateBundle checks that every file listed in the manifest exists in
// the bundle and optionally verifies SHA-256 checksums.
func ValidateBundle(fsys fs.FS, checkSHA bool) []error {
	m, err := ReadManifest(fsys)
	if err != nil {
		return []error{err}
	}

	var errs []error

	for _, repo := range m.Repos {
		for _, f := range repo.Files {
			path := filepath.Join("repos", repo.Name, f)
			if checkSHA {
				if err := verifySHA(fsys, path, ""); err != nil {
					errs = append(errs, err)
				}
			} else {
				if _, err := fs.Stat(fsys, path); err != nil {
					errs = append(errs, fmt.Errorf("missing repo file %s: %w", path, err))
				}
			}
		}
	}

	for _, doc := range m.Docs {
		if checkSHA {
			if err := verifySHA(fsys, doc.LocalPath, doc.SHA); err != nil {
				errs = append(errs, err)
			}
		} else {
			if _, err := fs.Stat(fsys, doc.LocalPath); err != nil {
				errs = append(errs, fmt.Errorf("missing doc %s: %w", doc.LocalPath, err))
			}
		}
	}

	return errs
}

// FileChecksum returns the hex-encoded SHA-256 of a file in an fs.FS.
func FileChecksum(fsys fs.FS, path string) (string, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h), nil
}

func verifySHA(fsys fs.FS, path, expected string) error {
	if _, err := fs.Stat(fsys, path); err != nil {
		return fmt.Errorf("missing %s: %w", path, err)
	}
	if expected == "" {
		return nil
	}
	got, err := FileChecksum(fsys, path)
	if err != nil {
		return fmt.Errorf("checksum %s: %w", path, err)
	}
	if got != expected {
		return fmt.Errorf("checksum mismatch %s: want %s, got %s", path, expected, got)
	}
	return nil
}

// CollectFiles walks a directory inside an fs.FS and returns sorted
// relative paths for all regular files. Useful for building RepoEntry.Files.
func CollectFiles(fsys fs.FS, root string) ([]string, error) {
	var files []string
	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
