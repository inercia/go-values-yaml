package yaml

import "testing"

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
