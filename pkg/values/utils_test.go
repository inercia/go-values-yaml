package values

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	yamllib "github.com/inercia/go-values-yaml/pkg/yaml"
	"github.com/psanford/memfs"
	syaml "sigs.k8s.io/yaml"
)

// Test utilities for filesystem operations

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir for write: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return data
}

// assertYAMLEqual compares YAML by unmarshaling and deep comparing.
func assertYAMLEqual(t *testing.T, expect, got []byte) {
	t.Helper()
	var ev any
	var gv any
	if err := syaml.Unmarshal(expect, &ev); err != nil {
		t.Fatalf("unmarshal expect: %v", err)
	}
	if err := syaml.Unmarshal(got, &gv); err != nil {
		t.Fatalf("unmarshal got: %v", err)
	}
	if !reflect.DeepEqual(ev, gv) {
		t.Fatalf("YAML not equal\nexpect:\n%s\ngot:\n%s", expect, got)
	}
}

// validateMergeProperty verifies that merge(common, updated) == original for Helm compatibility
func validateMergeProperty(t *testing.T, original, common, updated []byte) {
	t.Helper()
	reconstructed, err := yamllib.MergeYAML(common, updated)
	if err != nil {
		t.Fatalf("merge back failed: %v", err)
	}
	assertYAMLEqual(t, original, reconstructed)
}

// Test utilities for memfs operations

// memfsOps implements fileOps on top of github.com/psanford/memfs for use in tests.
type memfsOps struct{ fsys *memfs.FS }

func (m memfsOps) Stat(name string) (fs.FileInfo, error)        { return fs.Stat(m.fsys, name) }
func (m memfsOps) ReadFile(name string) ([]byte, error)         { return fs.ReadFile(m.fsys, name) }
func (m memfsOps) WalkDir(root string, fn fs.WalkDirFunc) error { return fs.WalkDir(m.fsys, root, fn) }
func (m memfsOps) WriteFileAtomic(path string, data []byte, perm fs.FileMode) error {
	if err := m.fsys.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return m.fsys.WriteFile(path, data, perm)
}

func writeMemFile(t *testing.T, mfs *memfs.FS, path string, data []byte) {
	t.Helper()
	if err := mfs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := mfs.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func readMemFile(t *testing.T, mfs *memfs.FS, path string) []byte {
	t.Helper()
	b, err := fs.ReadFile(mfs, path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return b
}

// Test setup helpers

// setupTempDirs creates a temporary directory structure for testing
func setupTempDirs(t *testing.T, paths ...string) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	fullPaths := make([]string, len(paths))
	for i, p := range paths {
		fullPath := filepath.Join(dir, p)
		mustMkdirAll(t, fullPath)
		fullPaths[i] = fullPath
	}
	return dir, fullPaths
}

// setupValuesFiles creates values.yaml files in the given directories with the provided content
func setupValuesFiles(t *testing.T, dirs []string, contents [][]byte) []string {
	t.Helper()
	if len(dirs) != len(contents) {
		t.Fatalf("dirs and contents length mismatch: %d vs %d", len(dirs), len(contents))
	}
	
	paths := make([]string, len(dirs))
	for i, dir := range dirs {
		path := filepath.Join(dir, "values.yaml")
		mustWriteFile(t, path, contents[i])
		paths[i] = path
	}
	return paths
}

// assertFileDoesNotExist verifies that a file does not exist
func assertFileDoesNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("unexpected file exists: %s", path)
	}
}

// assertTestFileExists verifies that a file exists (test helper)
func assertTestFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %s", path)
	}
}

// makeReadOnly makes a directory read-only for testing error conditions
func makeReadOnly(t *testing.T, path string) func() {
	t.Helper()
	if err := os.Chmod(path, 0o555); err != nil {
		t.Skipf("chmod failed, skipping: %v", err)
	}
	return func() { _ = os.Chmod(path, 0o750) }
}
