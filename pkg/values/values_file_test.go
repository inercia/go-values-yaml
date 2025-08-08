package values

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
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
	if err != nil {
		t.Fatalf("ExtractCommon error: %v", err)
	}

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

func TestExtractCommonN_CreatesCommonAndUpdatesAll(t *testing.T) {
	dir := t.TempDir()
	bdir := filepath.Join(dir, "a", "b")
	d1 := filepath.Join(bdir, "x")
	d2 := filepath.Join(bdir, "y")
	d3 := filepath.Join(bdir, "z")
	mustMkdirAll(t, d1)
	mustMkdirAll(t, d2)
	mustMkdirAll(t, d3)

	p1 := filepath.Join(d1, "values.yaml")
	p2 := filepath.Join(d2, "values.yaml")
	p3 := filepath.Join(d3, "values.yaml")

	y1 := []byte(`foo:
  bar:
    a: 1
    same: true
`)
	y2 := []byte(`foo:
  bar:
    b: 2
    same: true
`)
	y3 := []byte(`foo:
  bar:
    c: 3
    same: true
`)

	mustWriteFile(t, p1, y1)
	mustWriteFile(t, p2, y2)
	mustWriteFile(t, p3, y3)

	commonPath, err := ExtractCommonN([]string{p1, p2, p3})
	if err != nil {
		t.Fatalf("ExtractCommonN error: %v", err)
	}
	if commonPath != filepath.Join(bdir, "values.yaml") {
		t.Fatalf("unexpected common path: %s", commonPath)
	}

	assertYAMLEqual(t, []byte(`foo:
  bar:
    same: true
`), mustReadFile(t, commonPath))
	assertYAMLEqual(t, []byte(`foo:
  bar:
    a: 1
`), mustReadFile(t, p1))
	assertYAMLEqual(t, []byte(`foo:
  bar:
    b: 2
`), mustReadFile(t, p2))
	assertYAMLEqual(t, []byte(`foo:
  bar:
    c: 3
`), mustReadFile(t, p3))
}

func TestExtractCommonN_NoCommon_ReturnsErrAndNoChanges(t *testing.T) {
	dir := t.TempDir()
	bdir := filepath.Join(dir, "a", "b")
	d1 := filepath.Join(bdir, "x")
	d2 := filepath.Join(bdir, "y")
	mustMkdirAll(t, d1)
	mustMkdirAll(t, d2)

	p1 := filepath.Join(d1, "values.yaml")
	p2 := filepath.Join(d2, "values.yaml")
	mustWriteFile(t, p1, []byte(`a: 1`))
	mustWriteFile(t, p2, []byte(`b: 2`))

	_, err := ExtractCommonN([]string{p1, p2})
	if err == nil {
		t.Fatalf("expected ErrNoCommon, got nil")
	}
	if err != ErrNoCommon {
		t.Fatalf("expected ErrNoCommon, got %v", err)
	}

	// unchanged
	assertYAMLEqual(t, []byte("a: 1\n"), mustReadFile(t, p1))
	assertYAMLEqual(t, []byte("b: 2\n"), mustReadFile(t, p2))
}

func TestExtractCommon_NotSiblings_Error(t *testing.T) {
	dir := t.TempDir()
	adir := filepath.Join(dir, "a", "x")
	bdir := filepath.Join(dir, "b", "y")
	mustMkdirAll(t, adir)
	mustMkdirAll(t, bdir)

	p1 := filepath.Join(adir, "values.yaml")
	p2 := filepath.Join(bdir, "values.yaml")
	mustWriteFile(t, p1, []byte("a: 1\n"))
	mustWriteFile(t, p2, []byte("a: 1\n"))

	_, err := ExtractCommon(p1, p2)
	if err == nil {
		t.Fatalf("expected error for non-sibling files, got nil")
	}
	// ensure no common file written in either parent
	if _, err := os.Stat(filepath.Join(filepath.Dir(adir), "values.yaml")); err == nil {
		t.Fatalf("unexpected common file created under %s", filepath.Dir(adir))
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(bdir), "values.yaml")); err == nil {
		t.Fatalf("unexpected common file created under %s", filepath.Dir(bdir))
	}
}

func TestExtractCommon_ParentUnwritable_ErrorNoChanges(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "parent")
	x := filepath.Join(parent, "x")
	y := filepath.Join(parent, "y")
	mustMkdirAll(t, x)
	mustMkdirAll(t, y)
	p1 := filepath.Join(x, "values.yaml")
	p2 := filepath.Join(y, "values.yaml")
	orig1 := []byte("a: 1\n")
	orig2 := []byte("a: 1\n")
	mustWriteFile(t, p1, orig1)
	mustWriteFile(t, p2, orig2)

	// Make parent read-only to block common file creation
	if err := os.Chmod(parent, 0o555); err != nil { //nolint:gosec // tests intentionally set directory perms
		t.Skipf("chmod failed, skipping: %v", err)
	}
	defer func() { _ = os.Chmod(parent, 0o750) }() //nolint:gosec // restoring directory perms for tests

	_, err := ExtractCommon(p1, p2)
	if err == nil {
		t.Fatalf("expected error due to unwritable parent, got nil")
	}
	// Ensure children unchanged
	assertYAMLEqual(t, orig1, mustReadFile(t, p1))
	assertYAMLEqual(t, orig2, mustReadFile(t, p2))
	// Ensure no common file exists
	if _, err := os.Stat(filepath.Join(parent, "values.yaml")); err == nil {
		t.Fatalf("unexpected common file created in unwritable parent")
	}
}

func TestExtractCommonN_NotSiblings_Error(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a", "x", "values.yaml")
	p2 := filepath.Join(dir, "a", "y", "values.yaml")
	p3 := filepath.Join(dir, "b", "z", "values.yaml")
	mustWriteFile(t, p1, []byte("a: 1\n"))
	mustWriteFile(t, p2, []byte("a: 1\n"))
	mustWriteFile(t, p3, []byte("a: 1\n"))

	_, err := ExtractCommonN([]string{p1, p2, p3})
	if err == nil {
		t.Fatalf("expected error for non-sibling N files, got nil")
	}
}

func TestExtractCommonN_ParentUnwritable_ErrorNoChanges(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "parent")
	paths := []string{
		filepath.Join(parent, "x", "values.yaml"),
		filepath.Join(parent, "y", "values.yaml"),
		filepath.Join(parent, "z", "values.yaml"),
	}
	for _, p := range paths {
		mustWriteFile(t, p, []byte("k: v\n"))
	}
	if err := os.Chmod(parent, 0o555); err != nil { //nolint:gosec // tests intentionally set directory perms
		t.Skipf("chmod failed, skipping: %v", err)
	}
	defer func() { _ = os.Chmod(parent, 0o750) }() //nolint:gosec // restoring directory perms for tests

	_, err := ExtractCommonN(paths)
	if err == nil {
		t.Fatalf("expected error due to unwritable parent, got nil")
	}
	for _, p := range paths {
		assertYAMLEqual(t, []byte("k: v\n"), mustReadFile(t, p))
	}
	if _, err := os.Stat(filepath.Join(parent, "values.yaml")); err == nil {
		t.Fatalf("unexpected common file created in unwritable parent")
	}
}

func TestExtractCommonRecursive_SingleParentGroup(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "apps")
	d1 := filepath.Join(parent, "svc-a")
	d2 := filepath.Join(parent, "svc-b")
	mustMkdirAll(t, d1)
	mustMkdirAll(t, d2)

	p1 := filepath.Join(d1, "values.yaml")
	p2 := filepath.Join(d2, "values.yaml")

	y1 := []byte(`foo:
  bar:
    a: 1
    common: yes
`)
	y2 := []byte(`foo:
  bar:
    b: 2
    common: yes
`)
	mustWriteFile(t, p1, y1)
	mustWriteFile(t, p2, y2)

	created, err := ExtractCommonRecursive(dir)
	if err != nil {
		t.Fatalf("ExtractCommonRecursive error: %v", err)
	}
	expectCreated := []string{filepath.Join(parent, "values.yaml")}
	if !reflect.DeepEqual(expectCreated, created) {
		t.Fatalf("unexpected created list: %v", created)
	}

	assertYAMLEqual(t, []byte(`foo:
  bar:
    common: yes
`), mustReadFile(t, filepath.Join(parent, "values.yaml")))
	assertYAMLEqual(t, []byte(`foo:
  bar:
    a: 1
`), mustReadFile(t, p1))
	assertYAMLEqual(t, []byte(`foo:
  bar:
    b: 2
`), mustReadFile(t, p2))
}

func TestExtractCommonRecursive_MultipleParents(t *testing.T) {
	dir := t.TempDir()
	b := filepath.Join(dir, "env", "prod")
	c := filepath.Join(dir, "env", "staging")
	b1 := filepath.Join(b, "app1")
	b2 := filepath.Join(b, "app2")
	c1 := filepath.Join(c, "app3")
	c2 := filepath.Join(c, "app4")
	mustMkdirAll(t, b1)
	mustMkdirAll(t, b2)
	mustMkdirAll(t, c1)
	mustMkdirAll(t, c2)

	mustWriteFile(t, filepath.Join(b1, "values.yaml"), []byte(`cfg:
  image: v1
  replicas: 2
`))
	mustWriteFile(t, filepath.Join(b2, "values.yaml"), []byte(`cfg:
  image: v1
  replicas: 3
`))
	mustWriteFile(t, filepath.Join(c1, "values.yaml"), []byte(`cfg:
  image: v2
  replicas: 5
`))
	mustWriteFile(t, filepath.Join(c2, "values.yaml"), []byte(`cfg:
  image: v2
  replicas: 1
`))

	created, err := ExtractCommonRecursive(dir)
	if err != nil {
		t.Fatalf("ExtractCommonRecursive error: %v", err)
	}
	expect := []string{filepath.Join(b, "values.yaml"), filepath.Join(c, "values.yaml")}
	if !reflect.DeepEqual(expect, created) {
		t.Fatalf("unexpected created: %v", created)
	}

	assertYAMLEqual(t, []byte(`cfg:
  image: v1
`), mustReadFile(t, filepath.Join(b, "values.yaml")))
	assertYAMLEqual(t, []byte(`cfg:
  replicas: 2
`), mustReadFile(t, filepath.Join(b1, "values.yaml")))
	assertYAMLEqual(t, []byte(`cfg:
  replicas: 3
`), mustReadFile(t, filepath.Join(b2, "values.yaml")))

	assertYAMLEqual(t, []byte(`cfg:
  image: v2
`), mustReadFile(t, filepath.Join(c, "values.yaml")))
	assertYAMLEqual(t, []byte(`cfg:
  replicas: 5
`), mustReadFile(t, filepath.Join(c1, "values.yaml")))
	assertYAMLEqual(t, []byte(`cfg:
  replicas: 1
`), mustReadFile(t, filepath.Join(c2, "values.yaml")))
}

func TestExtractCommonRecursive_NoCommonGroup_Skip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "group")
	d1 := filepath.Join(p, "x")
	d2 := filepath.Join(p, "y")
	mustMkdirAll(t, d1)
	mustMkdirAll(t, d2)

	p1 := filepath.Join(d1, "values.yaml")
	p2 := filepath.Join(d2, "values.yaml")
	mustWriteFile(t, p1, []byte("a: 1\n"))
	mustWriteFile(t, p2, []byte("b: 2\n"))

	created, err := ExtractCommonRecursive(dir)
	if err != nil {
		t.Fatalf("ExtractCommonRecursive error: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("expected no parents created, got %v", created)
	}
	assertYAMLEqual(t, []byte("a: 1\n"), mustReadFile(t, p1))
	assertYAMLEqual(t, []byte("b: 2\n"), mustReadFile(t, p2))
}

func TestExtractCommonRecursive_UpwardPropagation_TwoLevels(t *testing.T) {
	dir := t.TempDir()
	// Tree:
	// /root/a/
	//   b/
	//     x/values.yaml   y/values.yaml (share common1)
	//   c/
	//     u/values.yaml   v/values.yaml (share common1)
	// Expect: create /root/a/b/values.yaml and /root/a/c/values.yaml then, since b and c are siblings under a, create /root/a/values.yaml with the common across (b,c)
	b := filepath.Join(dir, "a", "b")
	c := filepath.Join(dir, "a", "c")
	x := filepath.Join(b, "x")
	y := filepath.Join(b, "y")
	u := filepath.Join(c, "u")
	v := filepath.Join(c, "v")
	for _, d := range []string{x, y, u, v} {
		mustMkdirAll(t, d)
	}

	// Define children under b sharing some common and children under c sharing same common key/value
	mustWriteFile(t, filepath.Join(x, "values.yaml"), []byte(`svc:
  team: core
  image: v1
  replicas: 1
`))
	mustWriteFile(t, filepath.Join(y, "values.yaml"), []byte(`svc:
  team: core
  image: v1
  replicas: 2
`))
	mustWriteFile(t, filepath.Join(u, "values.yaml"), []byte(`svc:
  team: core
  image: v2
  replicas: 3
`))
	mustWriteFile(t, filepath.Join(v, "values.yaml"), []byte(`svc:
  team: core
  image: v2
  replicas: 4
`))

	created, err := ExtractCommonRecursive(dir)
	if err != nil {
		t.Fatalf("ExtractCommonRecursive error: %v", err)
	}

	// Expect 3 created parents: a/b, a/c, and a
	expect := []string{
		filepath.Join(filepath.Dir(b), "values.yaml"), // /a/values.yaml
		filepath.Join(b, "values.yaml"),
		filepath.Join(c, "values.yaml"),
	}
	// Sort both for deterministic compare
	sort.Strings(expect)
	if !reflect.DeepEqual(expect, created) {
		t.Fatalf("unexpected created: %v", created)
	}

	// Validate child remainders under a/b
	assertYAMLEqual(t, []byte(`svc:
  replicas: 1
`), mustReadFile(t, filepath.Join(x, "values.yaml")))
	assertYAMLEqual(t, []byte(`svc:
  replicas: 2
`), mustReadFile(t, filepath.Join(y, "values.yaml")))

	// Validate child remainders under a/c
	assertYAMLEqual(t, []byte(`svc:
  replicas: 3
`), mustReadFile(t, filepath.Join(u, "values.yaml")))
	assertYAMLEqual(t, []byte(`svc:
  replicas: 4
`), mustReadFile(t, filepath.Join(v, "values.yaml")))

	// Now a level: the common across (b,c) is team: core only
	assertYAMLEqual(t, []byte(`svc:
  team: core
`), mustReadFile(t, filepath.Join(filepath.Dir(b), "values.yaml")))

	// And a/b and a/c have remainders without the team
	assertYAMLEqual(t, []byte(`svc:
  image: v1
`), mustReadFile(t, filepath.Join(b, "values.yaml")))
	assertYAMLEqual(t, []byte(`svc:
  image: v2
`), mustReadFile(t, filepath.Join(c, "values.yaml")))
}

func TestExtractCommonRecursive_UpwardStopsOnNoCommon(t *testing.T) {
	dir := t.TempDir()
	// Build a case where first level produces parents, but top level has no common across those parents
	p := filepath.Join(dir, "apps")
	g1 := filepath.Join(p, "group1")
	g2 := filepath.Join(p, "group2")
	a1 := filepath.Join(g1, "a1")
	a2 := filepath.Join(g1, "a2")
	b1 := filepath.Join(g2, "b1")
	b2 := filepath.Join(g2, "b2")
	for _, d := range []string{a1, a2, b1, b2} {
		mustMkdirAll(t, d)
	}

	mustWriteFile(t, filepath.Join(a1, "values.yaml"), []byte(`data:
  region: us
  color: blue
`))
	mustWriteFile(t, filepath.Join(a2, "values.yaml"), []byte(`data:
  region: us
  color: red
`))
	mustWriteFile(t, filepath.Join(b1, "values.yaml"), []byte(`data:
  region: eu
  color: green
`))
	mustWriteFile(t, filepath.Join(b2, "values.yaml"), []byte(`data:
  region: eu
  color: yellow
`))

	created, err := ExtractCommonRecursive(dir)
	if err != nil {
		t.Fatalf("ExtractCommonRecursive error: %v", err)
	}

	// We expect parents at group1 and group2, but no common at apps level
	expect := []string{filepath.Join(g1, "values.yaml"), filepath.Join(g2, "values.yaml")}
	sort.Strings(expect)
	if !reflect.DeepEqual(expect, created) {
		t.Fatalf("unexpected created: %v", created)
	}

	assertYAMLEqual(t, []byte(`data:
  region: us
`), mustReadFile(t, filepath.Join(g1, "values.yaml")))
	assertYAMLEqual(t, []byte(`data:
  color: blue
`), mustReadFile(t, filepath.Join(a1, "values.yaml")))
	assertYAMLEqual(t, []byte(`data:
  color: red
`), mustReadFile(t, filepath.Join(a2, "values.yaml")))

	assertYAMLEqual(t, []byte(`data:
  region: eu
`), mustReadFile(t, filepath.Join(g2, "values.yaml")))
	assertYAMLEqual(t, []byte(`data:
  color: green
`), mustReadFile(t, filepath.Join(b1, "values.yaml")))
	assertYAMLEqual(t, []byte(`data:
  color: yellow
`), mustReadFile(t, filepath.Join(b2, "values.yaml")))

	// Ensure no apps-level values.yaml exists
	if _, err := os.Stat(filepath.Join(p, "values.yaml")); err == nil {
		t.Fatalf("unexpected top-level values.yaml created")
	}
}

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
