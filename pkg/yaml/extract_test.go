package yaml

import (
	"reflect"
	"testing"

	syaml "sigs.k8s.io/yaml"
)

func TestExtractCommon_SimpleMaps(t *testing.T) {
	tests := []struct {
		name         string
		y1, y2       []byte
		opts         []Option
		wantCommon   []byte
		wantUpdated1 []byte
		wantUpdated2 []byte
	}{
		{
			name: "simple maps with nested common subset",
			y1: []byte(`
foo:
  bar:
    something: [1,2,3]
    other: true
`),
			y2: []byte(`
foo:
  bar:
    else: [1,2,3]
    other: true
`),
			wantCommon: []byte(`foo:
  bar:
    other: true
`),
			wantUpdated1: []byte(`foo:
  bar:
    something:
    - 1
    - 2
    - 3
`),
			wantUpdated2: []byte(`foo:
  bar:
    else:
    - 1
    - 2
    - 3
`),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			common, u1, u2, err := ExtractCommon(tc.y1, tc.y2, tc.opts...)
			if err != nil {
				t.Fatalf("ExtractCommon error: %v", err)
			}
			assertYAMLEqual(t, tc.wantCommon, common)
			assertYAMLEqual(t, tc.wantUpdated1, u1)
			assertYAMLEqual(t, tc.wantUpdated2, u2)

			m1, err := MergeYAML(u1, common)
			if err != nil {
				t.Fatalf("MergeYAML m1 error: %v", err)
			}
			m2, err := MergeYAML(u2, common)
			if err != nil {
				t.Fatalf("MergeYAML m2 error: %v", err)
			}
			assertYAMLEqual(t, tc.y1, m1)
			assertYAMLEqual(t, tc.y2, m2)
		})
	}
}

func TestExtractCommon_ListsEqual_InCommon_ByDefault(t *testing.T) {
	tests := []struct {
		name       string
		y1, y2     []byte
		opts       []Option
		wantCommon []byte
		wantU1     []byte
		wantU2     []byte
	}{
		{
			name:       "equal lists go to common by default",
			y1:         []byte("a: [1, 2, 3]"),
			y2:         []byte("a: [1, 2, 3]"),
			wantCommon: []byte("a:\n- 1\n- 2\n- 3\n"),
			wantU1:     []byte("{}\n"),
			wantU2:     []byte("{}\n"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			common, u1, u2, err := ExtractCommon(tc.y1, tc.y2, tc.opts...)
			if err != nil {
				t.Fatalf("ExtractCommon error: %v", err)
			}
			assertYAMLEqual(t, tc.wantCommon, common)
			assertYAMLEqual(t, tc.wantU1, u1)
			assertYAMLEqual(t, tc.wantU2, u2)
		})
	}
}

func TestExtractCommon_ListsEqual_NotInCommon_WithOption(t *testing.T) {
	tests := []struct {
		name       string
		y1, y2     []byte
		opts       []Option
		wantCommon []byte
		wantU1     []byte
		wantU2     []byte
	}{
		{
			name:       "equal lists remain in remainders when option disabled",
			y1:         []byte("a: [1, 2, 3]"),
			y2:         []byte("a: [1, 2, 3]"),
			opts:       []Option{WithIncludeEqualListsInCommon(false)},
			wantCommon: []byte("{}\n"),
			wantU1:     []byte("a:\n- 1\n- 2\n- 3\n"),
			wantU2:     []byte("a:\n- 1\n- 2\n- 3\n"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			common, u1, u2, err := ExtractCommon(tc.y1, tc.y2, tc.opts...)
			if err != nil {
				t.Fatalf("ExtractCommon error: %v", err)
			}
			assertYAMLEqual(t, tc.wantCommon, common)
			assertYAMLEqual(t, tc.wantU1, u1)
			assertYAMLEqual(t, tc.wantU2, u2)

			m1, err := MergeYAML(u1, common)
			if err != nil {
				t.Fatalf("MergeYAML m1 error: %v", err)
			}
			m2, err := MergeYAML(u2, common)
			if err != nil {
				t.Fatalf("MergeYAML m2 error: %v", err)
			}
			assertYAMLEqual(t, tc.y1, m1)
			assertYAMLEqual(t, tc.y2, m2)
		})
	}
}

func TestExtractCommon_Scalars(t *testing.T) {
	tests := []struct {
		name       string
		y1, y2     []byte
		wantCommon []byte
		wantU1     []byte
		wantU2     []byte
	}{
		{
			name:       "equal scalars go to common",
			y1:         []byte("a: 5\n"),
			y2:         []byte("a: 5\n"),
			wantCommon: []byte("a: 5\n"),
			wantU1:     []byte("{}\n"),
			wantU2:     []byte("{}\n"),
		},
		{
			name:       "different scalars remain in remainders",
			y1:         []byte("a: 5\n"),
			y2:         []byte("a: 7\n"),
			wantCommon: []byte("{}\n"),
			wantU1:     []byte("a: 5\n"),
			wantU2:     []byte("a: 7\n"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			common, u1, u2, err := ExtractCommon(tc.y1, tc.y2)
			if err != nil {
				t.Fatal(err)
			}
			assertYAMLEqual(t, tc.wantCommon, common)
			assertYAMLEqual(t, tc.wantU1, u1)
			assertYAMLEqual(t, tc.wantU2, u2)
		})
	}
}

func TestExtractCommon_NestedStructures(t *testing.T) {
	tests := []struct {
		name       string
		y1, y2     []byte
		wantCommon []byte
	}{
		{
			name: "nested structures with partial common",
			y1: []byte(`
parent:
  child:
    name: app
    replicas: 2
    ports:
      - 80
      - 443
`),
			y2: []byte(`
parent:
  child:
    name: app
    replicas: 3
    ports:
      - 80
      - 8080
`),
			wantCommon: []byte(`parent:
  child:
    name: app
`),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			common, u1, u2, err := ExtractCommon(tc.y1, tc.y2)
			if err != nil {
				t.Fatal(err)
			}
			assertYAMLEqual(t, tc.wantCommon, common)

			m1, err := MergeYAML(u1, common)
			if err != nil {
				t.Fatal(err)
			}
			m2, err := MergeYAML(u2, common)
			if err != nil {
				t.Fatal(err)
			}
			assertYAMLEqual(t, tc.y1, m1)
			assertYAMLEqual(t, tc.y2, m2)
		})
	}
}

func TestExtractCommon_NoCommon_DisjointKeys(t *testing.T) {
	tests := []struct {
		name       string
		y1, y2     []byte
		wantCommon []byte
		wantU1     []byte
		wantU2     []byte
	}{
		{
			name: "disjoint top-level maps -> no common",
			y1: []byte(`
a: 1
b:
  c: 2
`),
			y2: []byte(`
x: true
y:
  z: 3
`),
			wantCommon: []byte("{}\n"),
			wantU1:     []byte("a: 1\n" + "b:\n  c: 2\n"),
			wantU2:     []byte("x: true\n" + "y:\n  z: 3\n"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			common, u1, u2, err := ExtractCommon(tc.y1, tc.y2)
			if err != nil {
				t.Fatal(err)
			}
			assertYAMLEqual(t, tc.wantCommon, common)
			assertYAMLEqual(t, tc.wantU1, u1)
			assertYAMLEqual(t, tc.wantU2, u2)
		})
	}
}

func TestExtractCommon_NoCommon_DifferentTopLevelLists(t *testing.T) {
	tests := []struct {
		name       string
		y1, y2     []byte
		wantCommon []byte
		wantU1     []byte
		wantU2     []byte
	}{
		{
			name:       "different top-level lists -> no common",
			y1:         []byte("\n- 1\n- 2\n"),
			y2:         []byte("\n- 3\n- 4\n"),
			wantCommon: []byte("{}\n"),
			wantU1:     []byte("- 1\n- 2\n"),
			wantU2:     []byte("- 3\n- 4\n"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			common, u1, u2, err := ExtractCommon(tc.y1, tc.y2)
			if err != nil {
				t.Fatal(err)
			}
			assertYAMLEqual(t, tc.wantCommon, common)
			assertYAMLEqual(t, tc.wantU1, u1)
			assertYAMLEqual(t, tc.wantU2, u2)
		})
	}
}

func TestExtractCommon_NoCommon_DifferentTypes(t *testing.T) {
	tests := []struct {
		name       string
		y1, y2     []byte
		wantCommon []byte
		wantU1     []byte
		wantU2     []byte
	}{
		{
			name:       "different YAML types -> no common",
			y1:         []byte("a: 1"),
			y2:         []byte("\n- item1\n- item2\n"),
			wantCommon: []byte("{}\n"),
			wantU1:     []byte("a: 1\n"),
			wantU2:     []byte("- item1\n- item2\n"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			common, u1, u2, err := ExtractCommon(tc.y1, tc.y2)
			if err != nil {
				t.Fatal(err)
			}
			assertYAMLEqual(t, tc.wantCommon, common)
			assertYAMLEqual(t, tc.wantU1, u1)
			assertYAMLEqual(t, tc.wantU2, u2)
		})
	}
}

func TestExtractCommonN_ThreeDocs_CommonAndRemainders(t *testing.T) {
	tests := []struct {
		name           string
		inputs         [][]byte
		wantCommon     []byte
		wantRemainders [][]byte
	}{
		{
			name: "three docs with shared subset",
			inputs: [][]byte{
				[]byte(`foo:
  bar:
    k1: v1
    same: true
`),
				[]byte(`foo:
  bar:
    k2: v2
    same: true
`),
				[]byte(`foo:
  bar:
    k3: v3
    same: true
`),
			},
			wantCommon: []byte(`foo:
  bar:
    same: true
`),
			wantRemainders: [][]byte{
				[]byte(`foo:
  bar:
    k1: v1
`),
				[]byte(`foo:
  bar:
    k2: v2
`),
				[]byte(`foo:
  bar:
    k3: v3
`),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			common, rems, err := ExtractCommonN(tc.inputs)
			if err != nil {
				t.Fatal(err)
			}
			assertYAMLEqual(t, tc.wantCommon, common)
			if len(rems) != len(tc.wantRemainders) {
				t.Fatalf("expected %d remainders, got %d", len(tc.wantRemainders), len(rems))
			}
			for i := range rems {
				assertYAMLEqual(t, tc.wantRemainders[i], rems[i])
				m, _ := MergeYAML(rems[i], common)
				assertYAMLEqual(t, tc.inputs[i], m)
			}
		})
	}
}

func TestExtractCommonN_Lists_AllEqual_DefaultInCommon(t *testing.T) {
	tests := []struct {
		name           string
		inputs         [][]byte
		wantCommon     []byte
		wantRemainders [][]byte
	}{
		{
			name: "lists all equal -> common contains list",
			inputs: [][]byte{
				[]byte(`a: [1,2,3]`),
				[]byte(`a: [1,2,3]`),
				[]byte(`a: [1,2,3]`),
			},
			wantCommon: []byte("a:\n- 1\n- 2\n- 3\n"),
			wantRemainders: [][]byte{
				[]byte("{}\n"), []byte("{}\n"), []byte("{}\n"),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			common, rems, err := ExtractCommonN(tc.inputs)
			if err != nil {
				t.Fatal(err)
			}
			assertYAMLEqual(t, tc.wantCommon, common)
			for i := range rems {
				assertYAMLEqual(t, tc.wantRemainders[i], rems[i])
			}
		})
	}
}

func TestExtractCommonN_Disjoint_NoCommon(t *testing.T) {
	tests := []struct {
		name           string
		inputs         [][]byte
		wantCommon     []byte
		wantRemainders [][]byte
	}{
		{
			name: "disjoint inputs -> empty common",
			inputs: [][]byte{
				[]byte(`a: 1`),
				[]byte(`b: 2`),
				[]byte(`c: 3`),
			},
			wantCommon: []byte("{}\n"),
			wantRemainders: [][]byte{
				[]byte("a: 1\n"),
				[]byte("b: 2\n"),
				[]byte("c: 3\n"),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			common, rems, err := ExtractCommonN(tc.inputs)
			if err != nil {
				t.Fatal(err)
			}
			assertYAMLEqual(t, tc.wantCommon, common)
			if len(rems) != len(tc.wantRemainders) {
				t.Fatalf("expected %d remainders, got %d", len(tc.wantRemainders), len(rems))
			}
			for i := range rems {
				assertYAMLEqual(t, tc.wantRemainders[i], rems[i])
			}
		})
	}
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
