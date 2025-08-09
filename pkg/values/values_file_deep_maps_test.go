package values

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestExtractCommon_DeepNestedMaps_OrderInsensitive(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "grp")
	a := filepath.Join(p, "a")
	b := filepath.Join(p, "b")
	mustMkdirAll(t, a)
	mustMkdirAll(t, b)

	p1 := filepath.Join(a, "values.yaml")
	p2 := filepath.Join(b, "values.yaml")

	y1 := []byte(`root:
  x:
    shared:
      deep:
        inner:
          alpha: 1
          beta: 2
  svc:
    image: a:v1
`)
	y2 := []byte(`root:
  svc:
    image: b:v1
  x:
    shared:
      deep:
        inner:
          beta: 2
          alpha: 1
`)

	mustWriteFile(t, p1, y1)
	mustWriteFile(t, p2, y2)

	commonPath, err := ExtractCommon(p1, p2)
	if err != nil {
		t.Fatalf("ExtractCommon error: %v", err)
	}
	if commonPath != filepath.Join(p, "values.yaml") {
		t.Fatalf("unexpected common path: %s", commonPath)
	}

	assertYAMLEqual(t, []byte(`root:
  x:
    shared:
      deep:
        inner:
          alpha: 1
          beta: 2
`), mustReadFile(t, commonPath))
	assertYAMLEqual(t, []byte(`root:
  svc:
    image: a:v1
`), mustReadFile(t, p1))
	assertYAMLEqual(t, []byte(`root:
  svc:
    image: b:v1
`), mustReadFile(t, p2))
}

func TestExtractCommonN_DeepNestedMaps_OrderInsensitive(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "apps")
	one := filepath.Join(p, "one")
	two := filepath.Join(p, "two")
	thr := filepath.Join(p, "three")
	mustMkdirAll(t, one)
	mustMkdirAll(t, two)
	mustMkdirAll(t, thr)

	p1 := filepath.Join(one, "values.yaml")
	p2 := filepath.Join(two, "values.yaml")
	p3 := filepath.Join(thr, "values.yaml")

	mustWriteFile(t, p1, []byte(`root:
  cfg:
    env: prod
  shared:
    deep:
      inner:
        k1: v1
        k2: v2
  svc:
    image: a:v1
`))
	mustWriteFile(t, p2, []byte(`root:
  svc:
    image: b:v1
  cfg:
    env: prod
  shared:
    deep:
      inner:
        k2: v2
        k1: v1
`))
	mustWriteFile(t, p3, []byte(`root:
  shared:
    deep:
      inner:
        k2: v2
        k1: v1
  svc:
    image: c:v1
  cfg:
    env: prod
`))

	cp, err := ExtractCommonN([]string{p1, p2, p3})
	if err != nil {
		t.Fatalf("ExtractCommonN error: %v", err)
	}
	if cp != filepath.Join(p, "values.yaml") {
		t.Fatalf("unexpected common path: %s", cp)
	}

	assertYAMLEqual(t, []byte(`root:
  cfg:
    env: prod
  shared:
    deep:
      inner:
        k1: v1
        k2: v2
`), mustReadFile(t, cp))
	assertYAMLEqual(t, []byte(`root:
  svc:
    image: a:v1
`), mustReadFile(t, p1))
	assertYAMLEqual(t, []byte(`root:
  svc:
    image: b:v1
`), mustReadFile(t, p2))
	assertYAMLEqual(t, []byte(`root:
  svc:
    image: c:v1
`), mustReadFile(t, p3))
}

func TestExtractCommonRecursive_DeepNestedMaps_OrderInsensitive(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	g1 := filepath.Join(root, "g1")
	g2 := filepath.Join(root, "g2")
	a := filepath.Join(g1, "a")
	b := filepath.Join(g1, "b")
	c := filepath.Join(g2, "c")
	d := filepath.Join(g2, "d")
	for _, ddir := range []string{a, b, c, d} {
		mustMkdirAll(t, ddir)
	}

	// g1 children
	mustWriteFile(t, filepath.Join(a, "values.yaml"), []byte(`root:
  shared:
    deep:
      inner:
        alpha: 1
        beta: 2
  svc:
    image: a:v1
`))
	mustWriteFile(t, filepath.Join(b, "values.yaml"), []byte(`root:
  svc:
    image: b:v1
  shared:
    deep:
      inner:
        beta: 2
        alpha: 1
`))
	// g2 children
	mustWriteFile(t, filepath.Join(c, "values.yaml"), []byte(`root:
  shared:
    deep:
      inner:
        alpha: 1
        beta: 2
  svc:
    image: c:v1
`))
	mustWriteFile(t, filepath.Join(d, "values.yaml"), []byte(`root:
  svc:
    image: d:v1
  shared:
    deep:
      inner:
        beta: 2
        alpha: 1
`))

	created, err := ExtractCommonRecursive(root)
	if err != nil {
		t.Fatalf("ExtractCommonRecursive error: %v", err)
	}

	expectCreated := []string{filepath.Join(g1, "values.yaml"), filepath.Join(g2, "values.yaml"), filepath.Join(root, "values.yaml")}
	sort.Strings(expectCreated)
	if !reflect.DeepEqual(expectCreated, created) {
		t.Fatalf("unexpected created list: %v", created)
	}

	// After top-level consolidation, group-level files become empty and common moves to root
	assertYAMLEqual(t, []byte(`{}
`), mustReadFile(t, filepath.Join(g1, "values.yaml")))
	assertYAMLEqual(t, []byte(`{}
`), mustReadFile(t, filepath.Join(g2, "values.yaml")))
	assertYAMLEqual(t, []byte(`root:
  shared:
    deep:
      inner:
        alpha: 1
        beta: 2
`), mustReadFile(t, filepath.Join(root, "values.yaml")))

	// Leaf remainders are unchanged after the top-level pass
	assertYAMLEqual(t, []byte(`root:
  svc:
    image: a:v1
`), mustReadFile(t, filepath.Join(a, "values.yaml")))
	assertYAMLEqual(t, []byte(`root:
  svc:
    image: b:v1
`), mustReadFile(t, filepath.Join(b, "values.yaml")))
	assertYAMLEqual(t, []byte(`root:
  svc:
    image: c:v1
`), mustReadFile(t, filepath.Join(c, "values.yaml")))
	assertYAMLEqual(t, []byte(`root:
  svc:
    image: d:v1
`), mustReadFile(t, filepath.Join(d, "values.yaml")))
}
