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

			m1, err := MergeYAML(common, u1)
			if err != nil {
				t.Fatalf("MergeYAML m1 error: %v", err)
			}
			m2, err := MergeYAML(common, u2)
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

			m1, err := MergeYAML(common, u1)
			if err != nil {
				t.Fatalf("MergeYAML m1 error: %v", err)
			}
			m2, err := MergeYAML(common, u2)
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

			m1, err := MergeYAML(common, u1)
			if err != nil {
				t.Fatal(err)
			}
			m2, err := MergeYAML(common, u2)
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
				m, _ := MergeYAML(common, rems[i])
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

func TestMergeYAML_HelmCompatibility_BasicAndNested(t *testing.T) {
	cases := []struct {
		name   string
		base   []byte
		over   []byte
		expect []byte
	}{
		{
			name: "scalar override and deep map merge",
			base: []byte(`
service:
  port: 80
  type: ClusterIP
feature: true
image: app:v1
`),
			over: []byte(`
service:
  type: NodePort
feature: false
image: app:v2
`),
			expect: []byte(`service:
  port: 80
  type: NodePort
feature: false
image: app:v2
`),
		},
		{
			name: "list replacement (not merge)",
			base: []byte(`
arr:
  - 1
  - 2
nested:
  arr:
    - a
    - b
`),
			over: []byte(`
arr:
  - 9
nested:
  arr:
    - x
`),
			expect: []byte(`arr:
- 9
nested:
  arr:
  - x
`),
		},
		{
			name: "type conflicts resolved by overlay (last wins)",
			base: []byte(`
obj:
  a: 1
list:
  - 1
num: 5
flag: true
`),
			over: []byte(`
obj: 42
list:
  x: 1
num:
  - 0
flag: false
`),
			expect: []byte(`obj: 42
list:
  x: 1
num:
- 0
flag: false
`),
		},
		{
			name: "null in overlay nullifies value (last wins)",
			base: []byte(`
foo:
  bar: 1
  baz: 2
`),
			over: []byte(`
foo:
  bar: null
`),
			expect: []byte(`foo:
  bar: null
  baz: 2
`),
		},
		{
			name: "top-level list replacement",
			base: []byte(`
- a
- b
`),
			over: []byte(`
- c
`),
			expect: []byte(`- c
`),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MergeYAML(tc.base, tc.over)
			if err != nil {
				t.Fatalf("MergeYAML error: %v", err)
			}
			assertYAMLEqual(t, tc.expect, got)
		})
	}
}

func TestMergeYAML_HelmCompatibility_MultiFileChain(t *testing.T) {
	base := []byte(`
a: 1
b:
  c: 1
`)
	over1 := []byte(`
a: 2
b:
  d: 2
`)
	over2 := []byte(`
b:
  c: 3
`)

	m1, err := MergeYAML(base, over1)
	if err != nil {
		panic(err)
	}
	got, err := MergeYAML(m1, over2)
	if err != nil {
		panic(err)
	}
	expect := []byte(`a: 2
b:
  c: 3
  d: 2
`)
	assertYAMLEqual(t, expect, got)
}

// Additional coverage tests
func TestExtractCommon_EmptyDocs_Normalization(t *testing.T) {
	tests := []struct {
		name       string
		y1, y2     []byte
		wantCommon []byte
		wantU1     []byte
		wantU2     []byte
	}{
		{
			name:       "both empty docs",
			y1:         []byte(""),
			y2:         []byte(""),
			wantCommon: []byte("{}\n"),
			wantU1:     []byte("{}\n"),
			wantU2:     []byte("{}\n"),
		},
		{
			name:       "first empty, second map",
			y1:         []byte(""),
			y2:         []byte("a: 1\n"),
			wantCommon: []byte("{}\n"),
			wantU1:     []byte("{}\n"),
			wantU2:     []byte("a: 1\n"),
		},
		{
			name:       "first map, second empty",
			y1:         []byte("a: 1\n"),
			y2:         []byte(""),
			wantCommon: []byte("{}\n"),
			wantU1:     []byte("a: 1\n"),
			wantU2:     []byte("{}\n"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			common, u1, u2, err := ExtractCommon(tc.y1, tc.y2)
			if err != nil {
				t.Fatalf("ExtractCommon error: %v", err)
			}
			assertYAMLEqual(t, tc.wantCommon, common)
			assertYAMLEqual(t, tc.wantU1, u1)
			assertYAMLEqual(t, tc.wantU2, u2)
			m1, _ := MergeYAML(common, u1)
			m2, _ := MergeYAML(common, u2)
			// Normalize empty originals to {} for comparison, since merge emits empty as {}
			expY1 := tc.y1
			expY2 := tc.y2
			if len(expY1) == 0 {
				expY1 = []byte("{}\n")
			}
			if len(expY2) == 0 {
				expY2 = []byte("{}\n")
			}
			assertYAMLEqual(t, expY1, m1)
			assertYAMLEqual(t, expY2, m2)
		})
	}
}

func TestExtractCommon_NestedListDifferences_NoPartial(t *testing.T) {
	y1 := []byte(`a:
  b:
  - 1
  - 2
  - 3
`)
	y2 := []byte(`a:
  b:
  - 1
  - 2
`)
	wantCommon := []byte("{}\n")
	wantU1 := []byte("a:\n  b:\n  - 1\n  - 2\n  - 3\n")
	wantU2 := []byte("a:\n  b:\n  - 1\n  - 2\n")

	common, u1, u2, err := ExtractCommon(y1, y2)
	if err != nil {
		t.Fatalf("ExtractCommon error: %v", err)
	}
	assertYAMLEqual(t, wantCommon, common)
	assertYAMLEqual(t, wantU1, u1)
	assertYAMLEqual(t, wantU2, u2)
	m1, _ := MergeYAML(common, u1)
	m2, _ := MergeYAML(common, u2)
	assertYAMLEqual(t, y1, m1)
	assertYAMLEqual(t, y2, m2)
}

func TestExtractCommon_NestedTypeConflict(t *testing.T) {
	y1 := []byte(`a:
  b:
    x: 1
`)
	y2 := []byte(`a:
  b: 2
`)
	wantCommon := []byte("{}\n")
	wantU1 := []byte("a:\n  b:\n    x: 1\n")
	wantU2 := []byte("a:\n  b: 2\n")

	common, u1, u2, err := ExtractCommon(y1, y2)
	if err != nil {
		t.Fatalf("ExtractCommon error: %v", err)
	}
	assertYAMLEqual(t, wantCommon, common)
	assertYAMLEqual(t, wantU1, u1)
	assertYAMLEqual(t, wantU2, u2)
	m1, _ := MergeYAML(common, u1)
	m2, _ := MergeYAML(common, u2)
	assertYAMLEqual(t, y1, m1)
	assertYAMLEqual(t, y2, m2)
}

func TestExtractCommonN_Lists_AllEqual_OptionDisabled(t *testing.T) {
	inputs := [][]byte{
		[]byte(`a: [1,2,3]`),
		[]byte(`a: [1,2,3]`),
		[]byte(`a: [1,2,3]`),
	}
	common, rems, err := ExtractCommonN(inputs, WithIncludeEqualListsInCommon(false))
	if err != nil {
		t.Fatalf("ExtractCommonN error: %v", err)
	}
	assertYAMLEqual(t, []byte("{}\n"), common)
	if len(rems) != 3 {
		t.Fatalf("expected 3 remainders, got %d", len(rems))
	}
	for i := range rems {
		assertYAMLEqual(t, inputs[i], rems[i])
	}
}

func TestExtractCommonN_KeyIntersection(t *testing.T) {
	inputs := [][]byte{
		[]byte(`a:
  x: 1
  y: 2
b: 1
`),
		[]byte(`a:
  x: 1
  z: 3
b: 2
`),
		[]byte(`a:
  x: 1
  y: 9
c: 3
`),
	}
	wantCommon := []byte("a:\n  x: 1\n")
	wantRemainders := [][]byte{
		[]byte("a:\n  y: 2\nb: 1\n"),
		[]byte("a:\n  z: 3\nb: 2\n"),
		[]byte("a:\n  y: 9\nc: 3\n"),
	}
	common, rems, err := ExtractCommonN(inputs)
	if err != nil {
		t.Fatalf("ExtractCommonN error: %v", err)
	}
	assertYAMLEqual(t, wantCommon, common)
	if len(rems) != len(wantRemainders) {
		t.Fatalf("expected %d remainders, got %d", len(wantRemainders), len(rems))
	}
	for i := range rems {
		assertYAMLEqual(t, wantRemainders[i], rems[i])
		m, _ := MergeYAML(common, rems[i])
		assertYAMLEqual(t, inputs[i], m)
	}
}

func TestExtractCommonN_OneEmpty_NoCommon(t *testing.T) {
	inputs := [][]byte{
		[]byte(""),
		[]byte("a: 1\n"),
		[]byte("a: 1\n"),
	}
	common, rems, err := ExtractCommonN(inputs)
	if err != nil {
		t.Fatalf("ExtractCommonN error: %v", err)
	}
	assertYAMLEqual(t, []byte("{}\n"), common)
	if len(rems) != 3 {
		t.Fatalf("expected 3 remainders, got %d", len(rems))
	}
	assertYAMLEqual(t, []byte("{}\n"), rems[0])
	assertYAMLEqual(t, []byte("a: 1\n"), rems[1])
	assertYAMLEqual(t, []byte("a: 1\n"), rems[2])
}
