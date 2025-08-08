package values

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	syaml "sigs.k8s.io/yaml"
)

func TestExtractCommon_CreatesCommonAndUpdatesChildren(t *testing.T) {
	dir := t.TempDir()
	// Create tree: /a/b/x/values.yaml and /a/b/y/values.yaml
	bdir := filepath.Join(dir, "a", "b")
	xdir := filepath.Join(bdir, "x")
	ydir := filepath.Join(bdir, "y")
	mustMkdirAll(t, xdir)
	mustMkdirAll(t, ydir)

	p1 := filepath.Join(xdir, "values.yaml")
	p2 := filepath.Join(ydir, "values.yaml")

	y1 := []byte(`foo:
  bar:
    something: [1,2,3]
    other: true
`)
	y2 := []byte(`foo:
  bar:
    else: [1,2,3]
    other: true
`)

	mustWriteFile(t, p1, y1)
	mustWriteFile(t, p2, y2)

	commonPath, err := ExtractCommon(p1, p2)
	if err != nil { t.Fatalf("ExtractCommon error: %v", err) }

	if commonPath != filepath.Join(bdir, "values.yaml") {
		t.Fatalf("unexpected common path: %s", commonPath)
	}

	commonData := mustReadFile(t, commonPath)
	expectCommon := []byte(`foo:
  bar:
    other: true
`)
	assertYAMLEqual(t, expectCommon, commonData)

	updated1 := mustReadFile(t, p1)
	updated2 := mustReadFile(t, p2)
	assertYAMLEqual(t, []byte(`foo:
  bar:
    something:
    - 1
    - 2
    - 3
`), updated1)
	assertYAMLEqual(t, []byte(`foo:
  bar:
    else:
    - 1
    - 2
    - 3
`), updated2)
}

func TestExtractCommon_NoCommon_ReturnsErrAndNoChanges(t *testing.T) {
	dir := t.TempDir()
	bdir := filepath.Join(dir, "a", "b")
	p := filepath.Join(bdir, "x")
	q := filepath.Join(bdir, "y")
	mustMkdirAll(t, p)
	mustMkdirAll(t, q)

	p1 := filepath.Join(p, "values.yaml")
	p2 := filepath.Join(q, "values.yaml")

	y1 := []byte(`a: 1`)
	y2 := []byte(`b: 2`)
	mustWriteFile(t, p1, y1)
	mustWriteFile(t, p2, y2)

	_, err := ExtractCommon(p1, p2)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err != ErrNoCommon {
		t.Fatalf("expected ErrNoCommon, got %v", err)
	}

	// Ensure files unchanged
	assertYAMLEqual(t, y1, mustReadFile(t, p1))
	assertYAMLEqual(t, y2, mustReadFile(t, p2))
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for write: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil { t.Fatalf("read file: %v", err) }
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
