package yaml

import (
	"testing"

	syaml "sigs.k8s.io/yaml"
)

func TestExtractCommon_DeepNestedMaps_OrderInsensitive(t *testing.T) {
	y1 := []byte(`
root:
  a:
    shared:
      deep:
        inner:
          alpha: 1
          beta: 2
    svc:
      image: a:v1
`)
	y2 := []byte(`
root:
  a:
    svc:
      image: b:v1
    shared:
      deep:
        inner:
          beta: 2
          alpha: 1
`)

	common, u1, u2, err := ExtractCommon(y1, y2)
	if err != nil {
		t.Fatalf("ExtractCommon error: %v", err)
	}

	expectCommon := []byte(`root:
  a:
    shared:
      deep:
        inner:
          alpha: 1
          beta: 2
`)
	expectU1 := []byte(`root:
  a:
    svc:
      image: a:v1
`)
	expectU2 := []byte(`root:
  a:
    svc:
      image: b:v1
`)

	assertYAMLEqualDeep(t, expectCommon, common)
	assertYAMLEqualDeep(t, expectU1, u1)
	assertYAMLEqualDeep(t, expectU2, u2)

	m1, err := MergeYAML(u1, common)
	if err != nil {
		t.Fatalf("MergeYAML m1 error: %v", err)
	}
	m2, err := MergeYAML(u2, common)
	if err != nil {
		t.Fatalf("MergeYAML m2 error: %v", err)
	}
	assertYAMLEqualDeep(t, y1, m1)
	assertYAMLEqualDeep(t, y2, m2)
}

func TestExtractCommonN_DeepNestedMaps_OrderInsensitive(t *testing.T) {
	inputs := [][]byte{
		[]byte(`root:
  cfg:
    env: prod
  shared:
    deep:
      inner:
        k1: v1
        k2: v2
  svc:
    image: a:v1
`),
		[]byte(`root:
  cfg:
    env: prod
  svc:
    image: b:v1
  shared:
    deep:
      inner:
        k2: v2
        k1: v1
`),
		[]byte(`root:
  shared:
    deep:
      inner:
        k2: v2
        k1: v1
  cfg:
    env: prod
  svc:
    image: c:v1
`),
	}

	common, rems, err := ExtractCommonN(inputs)
	if err != nil {
		t.Fatalf("ExtractCommonN error: %v", err)
	}
	if len(rems) != 3 {
		t.Fatalf("expected 3 remainders, got %d", len(rems))
	}

	expectCommon := []byte(`root:
  cfg:
    env: prod
  shared:
    deep:
      inner:
        k1: v1
        k2: v2
`)
	assertYAMLEqualDeep(t, expectCommon, common)

	expectR1 := []byte(`root:
  svc:
    image: a:v1
`)
	expectR2 := []byte(`root:
  svc:
    image: b:v1
`)
	expectR3 := []byte(`root:
  svc:
    image: c:v1
`)
	assertYAMLEqualDeep(t, expectR1, rems[0])
	assertYAMLEqualDeep(t, expectR2, rems[1])
	assertYAMLEqualDeep(t, expectR3, rems[2])

	m0, err := MergeYAML(rems[0], common)
	if err != nil {
		t.Fatalf("merge 0: %v", err)
	}
	m1, err := MergeYAML(rems[1], common)
	if err != nil {
		t.Fatalf("merge 1: %v", err)
	}
	m2, err := MergeYAML(rems[2], common)
	if err != nil {
		t.Fatalf("merge 2: %v", err)
	}
	assertYAMLEqualDeep(t, inputs[0], m0)
	assertYAMLEqualDeep(t, inputs[1], m1)
	assertYAMLEqualDeep(t, inputs[2], m2)
}

// Local helper to avoid colliding with existing helper name in another file
func assertYAMLEqualDeep(t *testing.T, expect, got []byte) {
	t.Helper()
	var ev any
	var gv any
	if err := syaml.Unmarshal(expect, &ev); err != nil {
		t.Fatalf("unmarshal expect: %v", err)
	}
	if err := syaml.Unmarshal(got, &gv); err != nil {
		t.Fatalf("unmarshal got: %v", err)
	}
	if !deepEqual(ev, gv) {
		t.Fatalf("YAML not equal\nexpect:\n%s\ngot:\n%s", expect, got)
	}
}
