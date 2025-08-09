package yaml

import (
	"testing"

	syaml "sigs.k8s.io/yaml"
)

func TestExtractCommon_DeepNestedMaps_OrderInsensitive(t *testing.T) {
	tests := []struct {
		name       string
		y1, y2     []byte
		wantCommon []byte
		wantU1     []byte
		wantU2     []byte
	}{
		{
			name: "order-insensitive deep map extraction",
			y1: []byte(`
root:
  a:
    shared:
      deep:
        inner:
          alpha: 1
          beta: 2
    svc:
      image: a:v1
`),
			y2: []byte(`
root:
  a:
    svc:
      image: b:v1
    shared:
      deep:
        inner:
          beta: 2
          alpha: 1
`),
			wantCommon: []byte(`root:
  a:
    shared:
      deep:
        inner:
          alpha: 1
          beta: 2
`),
			wantU1: []byte(`root:
  a:
    svc:
      image: a:v1
`),
			wantU2: []byte(`root:
  a:
    svc:
      image: b:v1
`),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			common, u1, u2, err := ExtractCommon(tc.y1, tc.y2)
			if err != nil {
				t.Fatalf("ExtractCommon error: %v", err)
			}
			assertYAMLEqualDeep(t, tc.wantCommon, common)
			assertYAMLEqualDeep(t, tc.wantU1, u1)
			assertYAMLEqualDeep(t, tc.wantU2, u2)

			m1, err := MergeYAML(u1, common)
			if err != nil {
				t.Fatalf("MergeYAML m1 error: %v", err)
			}
			m2, err := MergeYAML(u2, common)
			if err != nil {
				t.Fatalf("MergeYAML m2 error: %v", err)
			}
			assertYAMLEqualDeep(t, tc.y1, m1)
			assertYAMLEqualDeep(t, tc.y2, m2)
		})
	}
}

func TestExtractCommonN_DeepNestedMaps_OrderInsensitive(t *testing.T) {
	tests := []struct {
		name           string
		inputs         [][]byte
		wantCommon     []byte
		wantRemainders [][]byte
	}{
		{
			name: "order-insensitive deep map extraction across N docs",
			inputs: [][]byte{
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
			},
			wantCommon: []byte(`root:
  cfg:
    env: prod
  shared:
    deep:
      inner:
        k1: v1
        k2: v2
`),
			wantRemainders: [][]byte{
				[]byte(`root:
  svc:
    image: a:v1
`),
				[]byte(`root:
  svc:
    image: b:v1
`),
				[]byte(`root:
  svc:
    image: c:v1
`),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			common, rems, err := ExtractCommonN(tc.inputs)
			if err != nil {
				t.Fatalf("ExtractCommonN error: %v", err)
			}
			if len(rems) != len(tc.wantRemainders) {
				t.Fatalf("expected %d remainders, got %d", len(tc.wantRemainders), len(rems))
			}
			assertYAMLEqualDeep(t, tc.wantCommon, common)
			for i := range rems {
				assertYAMLEqualDeep(t, tc.wantRemainders[i], rems[i])
				m, err := MergeYAML(rems[i], common)
				if err != nil {
					t.Fatalf("merge %d: %v", i, err)
				}
				assertYAMLEqualDeep(t, tc.inputs[i], m)
			}
		})
	}
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
