package yaml

import (
	"reflect"
	"testing"

	syaml "sigs.k8s.io/yaml"
)

func TestExtractCommon_SimpleMaps(t *testing.T) {
	y1 := []byte(`
foo:
  bar:
    something: [1,2,3]
    other: true
`)
	y2 := []byte(`
foo:
  bar:
    else: [1,2,3]
    other: true
`)

	common, u1, u2, err := ExtractCommon(y1, y2)
	if err != nil {
		t.Fatalf("ExtractCommon error: %v", err)
	}

	expectCommon := []byte(`foo:
  bar:
    other: true
`)
	expectU1 := []byte(`foo:
  bar:
    something:
    - 1
    - 2
    - 3
`)
	expectU2 := []byte(`foo:
  bar:
    else:
    - 1
    - 2
    - 3
`)

	assertYAMLEqual(t, expectCommon, common)
	assertYAMLEqual(t, expectU1, u1)
	assertYAMLEqual(t, expectU2, u2)

	// Merge property
	m1, err := MergeYAML(u1, common)
	if err != nil {
		t.Fatalf("MergeYAML m1 error: %v", err)
	}
	m2, err := MergeYAML(u2, common)
	if err != nil {
		t.Fatalf("MergeYAML m2 error: %v", err)
	}
	assertYAMLEqual(t, y1, m1)
	assertYAMLEqual(t, y2, m2)
}

func TestExtractCommon_ListsEqual_InCommon_ByDefault(t *testing.T) {
	y1 := []byte(`a: [1, 2, 3]`)
	y2 := []byte(`a: [1, 2, 3]`)

	common, u1, u2, err := ExtractCommon(y1, y2)
	if err != nil {
		t.Fatalf("ExtractCommon error: %v", err)
	}
	assertYAMLEqual(t, []byte("a:\n- 1\n- 2\n- 3\n"), common)
	assertYAMLEqual(t, []byte("{}\n"), u1)
	assertYAMLEqual(t, []byte("{}\n"), u2)
}

func TestExtractCommon_ListsEqual_NotInCommon_WithOption(t *testing.T) {
	y1 := []byte(`a: [1, 2, 3]`)
	y2 := []byte(`a: [1, 2, 3]`)

	common, u1, u2, err := ExtractCommon(y1, y2, WithIncludeEqualListsInCommon(false))
	if err != nil {
		t.Fatalf("ExtractCommon error: %v", err)
	}
	assertYAMLEqual(t, []byte("{}\n"), common)
	assertYAMLEqual(t, []byte("a:\n- 1\n- 2\n- 3\n"), u1)
	assertYAMLEqual(t, []byte("a:\n- 1\n- 2\n- 3\n"), u2)

	// Merge property
	m1, err := MergeYAML(u1, common)
	if err != nil {
		t.Fatalf("MergeYAML m1 error: %v", err)
	}
	m2, err := MergeYAML(u2, common)
	if err != nil {
		t.Fatalf("MergeYAML m2 error: %v", err)
	}
	assertYAMLEqual(t, y1, m1)
	assertYAMLEqual(t, y2, m2)
}

func TestExtractCommon_Scalars(t *testing.T) {
	// equal scalars -> common
	common, u1, u2, err := ExtractCommon([]byte("a: 5\n"), []byte("a: 5\n"))
	if err != nil {
		t.Fatal(err)
	}
	assertYAMLEqual(t, []byte("a: 5\n"), common)
	assertYAMLEqual(t, []byte("{}\n"), u1)
	assertYAMLEqual(t, []byte("{}\n"), u2)

	// different scalars -> in remainders
	common, u1, u2, err = ExtractCommon([]byte("a: 5\n"), []byte("a: 7\n"))
	if err != nil {
		t.Fatal(err)
	}
	assertYAMLEqual(t, []byte("{}\n"), common)
	assertYAMLEqual(t, []byte("a: 5\n"), u1)
	assertYAMLEqual(t, []byte("a: 7\n"), u2)
}

func TestExtractCommon_NestedStructures(t *testing.T) {
	y1 := []byte(`
parent:
  child:
    name: app
    replicas: 2
    ports:
      - 80
      - 443
`)
	y2 := []byte(`
parent:
  child:
    name: app
    replicas: 3
    ports:
      - 80
      - 8080
`)

	common, u1, u2, err := ExtractCommon(y1, y2)
	if err != nil {
		t.Fatal(err)
	}

	expectCommon := []byte(`parent:
  child:
    name: app
`)
	assertYAMLEqual(t, expectCommon, common)

	// Merge property
	m1, err := MergeYAML(u1, common)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := MergeYAML(u2, common)
	if err != nil {
		t.Fatal(err)
	}
	assertYAMLEqual(t, y1, m1)
	assertYAMLEqual(t, y2, m2)
}

func TestExtractCommon_NoCommon_DisjointKeys(t *testing.T) {
	y1 := []byte(`
a: 1
b:
  c: 2
`)
	y2 := []byte(`
x: true
y:
  z: 3
`)

	common, u1, u2, err := ExtractCommon(y1, y2)
	if err != nil {
		t.Fatal(err)
	}
	assertYAMLEqual(t, []byte("{}\n"), common)
	assertYAMLEqual(t, y1, u1)
	assertYAMLEqual(t, y2, u2)
}

func TestExtractCommon_NoCommon_DifferentTopLevelLists(t *testing.T) {
	y1 := []byte(`
- 1
- 2
`)
	y2 := []byte(`
- 3
- 4
`)

	common, u1, u2, err := ExtractCommon(y1, y2)
	if err != nil {
		t.Fatal(err)
	}
	assertYAMLEqual(t, []byte("{}\n"), common)
	assertYAMLEqual(t, y1, u1)
	assertYAMLEqual(t, y2, u2)
}

func TestExtractCommon_NoCommon_DifferentTypes(t *testing.T) {
	y1 := []byte(`a: 1`)
	y2 := []byte(`
- item1
- item2
`)
	common, u1, u2, err := ExtractCommon(y1, y2)
	if err != nil {
		t.Fatal(err)
	}
	assertYAMLEqual(t, []byte("{}\n"), common)
	assertYAMLEqual(t, y1, u1)
	assertYAMLEqual(t, y2, u2)
}

// assertYAMLEqual compares YAML by unmarshaling and deep comparing.
func assertYAMLEqual(t *testing.T, expect, got []byte) {
	t.Helper()
	var ev any
	var gv any
	if err := yamlToIface(expect, &ev); err != nil {
		t.Fatalf("unmarshal expect: %v", err)
	}
	if err := yamlToIface(got, &gv); err != nil {
		t.Fatalf("unmarshal got: %v", err)
	}
	if !deepEqual(ev, gv) {
		t.Fatalf("YAML not equal\nexpect:\n%s\ngot:\n%s", expect, got)
	}
}

func yamlToIface(b []byte, v *any) error {
	return syaml.Unmarshal(b, v)
}

func deepEqual(a, b any) bool {
	return reflect.DeepEqual(a, b)
}
