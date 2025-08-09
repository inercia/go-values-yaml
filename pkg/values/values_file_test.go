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
	tests := []struct {
		name         string
		y1, y2       []byte
		wantCommon   []byte
		wantUpdated1 []byte
		wantUpdated2 []byte
	}{
		{
			name: "nested common subset moved to parent; remainders kept",
			y1: []byte(`foo:
  bar:
    something: [1,2,3]
    other: true
`),
			y2: []byte(`foo:
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
			dir := t.TempDir()
			bdir := filepath.Join(dir, "a", "b")
			xdir := filepath.Join(bdir, "x")
			ydir := filepath.Join(bdir, "y")
			mustMkdirAll(t, xdir)
			mustMkdirAll(t, ydir)

			p1 := filepath.Join(xdir, "values.yaml")
			p2 := filepath.Join(ydir, "values.yaml")
			mustWriteFile(t, p1, tc.y1)
			mustWriteFile(t, p2, tc.y2)

			commonPath, err := ExtractCommon(p1, p2)
			if err != nil {
				t.Fatalf("ExtractCommon error: %v", err)
			}
			if commonPath != filepath.Join(bdir, "values.yaml") {
				t.Fatalf("unexpected common path: %s", commonPath)
			}
			assertYAMLEqual(t, tc.wantCommon, mustReadFile(t, commonPath))
			assertYAMLEqual(t, tc.wantUpdated1, mustReadFile(t, p1))
			assertYAMLEqual(t, tc.wantUpdated2, mustReadFile(t, p2))
		})
	}
}

func TestExtractCommon_NoCommon_ReturnsErrAndNoChanges(t *testing.T) {
	tests := []struct {
		name   string
		y1, y2 []byte
	}{
		{
			name: "no common keys -> returns ErrNoCommon and leaves files",
			y1:   []byte(`a: 1`),
			y2:   []byte(`b: 2`),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bdir := filepath.Join(dir, "a", "b")
			p := filepath.Join(bdir, "x")
			q := filepath.Join(bdir, "y")
			mustMkdirAll(t, p)
			mustMkdirAll(t, q)

			p1 := filepath.Join(p, "values.yaml")
			p2 := filepath.Join(q, "values.yaml")
			mustWriteFile(t, p1, tc.y1)
			mustWriteFile(t, p2, tc.y2)

			_, err := ExtractCommon(p1, p2)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if err != ErrNoCommon {
				t.Fatalf("expected ErrNoCommon, got %v", err)
			}

			assertYAMLEqual(t, tc.y1, mustReadFile(t, p1))
			assertYAMLEqual(t, tc.y2, mustReadFile(t, p2))
		})
	}
}

func TestExtractCommonN_CreatesCommonAndUpdatesAll(t *testing.T) {
	tests := []struct {
		name        string
		inputs      [][]byte
		wantCommon  []byte
		wantUpdates [][]byte
	}{
		{
			name: "N sibling files share subset",
			inputs: [][]byte{
				[]byte(`foo:
  bar:
    a: 1
    same: true
`),
				[]byte(`foo:
  bar:
    b: 2
    same: true
`),
				[]byte(`foo:
  bar:
    c: 3
    same: true
`),
			},
			wantCommon: []byte(`foo:
  bar:
    same: true
`),
			wantUpdates: [][]byte{
				[]byte(`foo:
  bar:
    a: 1
`),
				[]byte(`foo:
  bar:
    b: 2
`),
				[]byte(`foo:
  bar:
    c: 3
`),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bdir := filepath.Join(dir, "a", "b")
			dirs := []string{filepath.Join(bdir, "x"), filepath.Join(bdir, "y"), filepath.Join(bdir, "z")}
			for _, d := range dirs {
				mustMkdirAll(t, d)
			}
			paths := []string{filepath.Join(dirs[0], "values.yaml"), filepath.Join(dirs[1], "values.yaml"), filepath.Join(dirs[2], "values.yaml")}
			for i, p := range paths {
				mustWriteFile(t, p, tc.inputs[i])
			}
			commonPath, err := ExtractCommonN(paths)
			if err != nil {
				t.Fatalf("ExtractCommonN error: %v", err)
			}
			if commonPath != filepath.Join(bdir, "values.yaml") {
				t.Fatalf("unexpected common path: %s", commonPath)
			}
			assertYAMLEqual(t, tc.wantCommon, mustReadFile(t, commonPath))
			for i, p := range paths {
				assertYAMLEqual(t, tc.wantUpdates[i], mustReadFile(t, p))
			}
		})
	}
}

func TestExtractCommonN_NoCommon_ReturnsErrAndNoChanges(t *testing.T) {
	tests := []struct {
		name string
		in1  []byte
		in2  []byte
	}{
		{
			name: "two siblings with disjoint keys -> ErrNoCommon and unchanged",
			in1:  []byte(`a: 1`),
			in2:  []byte(`b: 2`),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bdir := filepath.Join(dir, "a", "b")
			d1 := filepath.Join(bdir, "x")
			d2 := filepath.Join(bdir, "y")
			mustMkdirAll(t, d1)
			mustMkdirAll(t, d2)

			p1 := filepath.Join(d1, "values.yaml")
			p2 := filepath.Join(d2, "values.yaml")
			mustWriteFile(t, p1, tc.in1)
			mustWriteFile(t, p2, tc.in2)

			_, err := ExtractCommonN([]string{p1, p2})
			if err == nil {
				t.Fatalf("expected ErrNoCommon, got nil")
			}
			if err != ErrNoCommon {
				t.Fatalf("expected ErrNoCommon, got %v", err)
			}
			assertYAMLEqual(t, tc.in1, mustReadFile(t, p1))
			assertYAMLEqual(t, tc.in2, mustReadFile(t, p2))
		})
	}
}

func TestExtractCommon_NotSiblings_Error(t *testing.T) {
	tests := []struct {
		name             string
		input1, input2   []byte
		wantCommonUnderA bool
		wantCommonUnderB bool
	}{
		{
			name:             "non-sibling files -> error and no common files created",
			input1:           []byte("a: 1\n"),
			input2:           []byte("a: 1\n"),
			wantCommonUnderA: false,
			wantCommonUnderB: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			adir := filepath.Join(dir, "a", "x")
			bdir := filepath.Join(dir, "b", "y")
			mustMkdirAll(t, adir)
			mustMkdirAll(t, bdir)

			p1 := filepath.Join(adir, "values.yaml")
			p2 := filepath.Join(bdir, "values.yaml")
			mustWriteFile(t, p1, tc.input1)
			mustWriteFile(t, p2, tc.input2)

			_, err := ExtractCommon(p1, p2)
			if err == nil {
				t.Fatalf("expected error for non-sibling files, got nil")
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(adir), "values.yaml")); (err == nil) != tc.wantCommonUnderA {
				t.Fatalf("unexpected common file presence under %s: got %v", filepath.Dir(adir), err == nil)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(bdir), "values.yaml")); (err == nil) != tc.wantCommonUnderB {
				t.Fatalf("unexpected common file presence under %s: got %v", filepath.Dir(bdir), err == nil)
			}
		})
	}
}

func TestExtractCommon_ParentUnwritable_ErrorNoChanges(t *testing.T) {
	tests := []struct {
		name   string
		leftY  []byte
		rightY []byte
	}{
		{
			name:   "parent unwritable -> error and no changes",
			leftY:  []byte("a: 1\n"),
			rightY: []byte("a: 1\n"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			parent := filepath.Join(dir, "parent")
			x := filepath.Join(parent, "x")
			y := filepath.Join(parent, "y")
			mustMkdirAll(t, x)
			mustMkdirAll(t, y)
			p1 := filepath.Join(x, "values.yaml")
			p2 := filepath.Join(y, "values.yaml")
			mustWriteFile(t, p1, tc.leftY)
			mustWriteFile(t, p2, tc.rightY)

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
			assertYAMLEqual(t, tc.leftY, mustReadFile(t, p1))
			assertYAMLEqual(t, tc.rightY, mustReadFile(t, p2))
			// Ensure no common file exists
			if _, err := os.Stat(filepath.Join(parent, "values.yaml")); err == nil {
				t.Fatalf("unexpected common file created in unwritable parent")
			}
		})
	}
}

func TestExtractCommonN_NotSiblings_Error(t *testing.T) {
	tests := []struct {
		name          string
		in1, in2, in3 []byte
	}{
		{
			name: "at least one of N files not sharing same parent -> error",
			in1:  []byte("a: 1\n"),
			in2:  []byte("a: 1\n"),
			in3:  []byte("a: 1\n"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p1 := filepath.Join(dir, "a", "x", "values.yaml")
			p2 := filepath.Join(dir, "a", "y", "values.yaml")
			p3 := filepath.Join(dir, "b", "z", "values.yaml")
			mustWriteFile(t, p1, tc.in1)
			mustWriteFile(t, p2, tc.in2)
			mustWriteFile(t, p3, tc.in3)

			_, err := ExtractCommonN([]string{p1, p2, p3})
			if err == nil {
				t.Fatalf("expected error for non-sibling N files, got nil")
			}
		})
	}
}

func TestExtractCommonN_ParentUnwritable_ErrorNoChanges(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
	}{
		{
			name:    "parent directory not writable -> error and no changes",
			content: []byte("k: v\n"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			parent := filepath.Join(dir, "parent")
			paths := []string{
				filepath.Join(parent, "x", "values.yaml"),
				filepath.Join(parent, "y", "values.yaml"),
				filepath.Join(parent, "z", "values.yaml"),
			}
			for _, p := range paths {
				mustWriteFile(t, p, tc.content)
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
				assertYAMLEqual(t, tc.content, mustReadFile(t, p))
			}
			if _, err := os.Stat(filepath.Join(parent, "values.yaml")); err == nil {
				t.Fatalf("unexpected common file created in unwritable parent")
			}
		})
	}
}

func TestExtractCommonRecursive_SingleParentGroup(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "sibling children with shared subset -> parent values.yaml created"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
		})
	}
}

func TestExtractCommonRecursive_MultipleParents(t *testing.T) {
	tests := []struct{ name string }{
		{name: "multiple groups under env -> per-group parents created"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
		})
	}
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

func TestExtractCommonRecursive_GrandchildrenSiblings_CommonAtGrandparent(t *testing.T) {
	dir := t.TempDir()
	// Layout:
	// /a/1/X/values.yaml and /a/2/Y/values.yaml
	a := filepath.Join(dir, "a")
	oneX := filepath.Join(a, "1", "X")
	twoY := filepath.Join(a, "2", "Y")
	mustMkdirAll(t, oneX)
	mustMkdirAll(t, twoY)

	p1 := filepath.Join(oneX, "values.yaml")
	p2 := filepath.Join(twoY, "values.yaml")

	// Similar nested structure with a common grandparent-level subset
	mustWriteFile(t, p1, []byte(`foo:
  bar:
    something: [1,2,3]
    other: true
`))
	mustWriteFile(t, p2, []byte(`foo:
  bar:
    else: [1,2,3]
    other: true
`))

	created, err := ExtractCommonRecursive(dir)
	if err != nil {
		t.Fatalf("ExtractCommonRecursive error: %v", err)
	}

	// Expect a single values.yaml created at /a
	expect := []string{filepath.Join(a, "values.yaml")}
	if !reflect.DeepEqual(expect, created) {
		t.Fatalf("unexpected created: %v", created)
	}

	// Validate common at /a and remainders at grandchildren
	assertYAMLEqual(t, []byte(`foo:
  bar:
    other: true
`), mustReadFile(t, filepath.Join(a, "values.yaml")))
	assertYAMLEqual(t, []byte(`foo:
  bar:
    something:
    - 1
    - 2
    - 3
`), mustReadFile(t, p1))
	assertYAMLEqual(t, []byte(`foo:
  bar:
    else:
    - 1
    - 2
    - 3
`), mustReadFile(t, p2))

	// Ensure no intermediate values.yaml were created at /a/1 or /a/2
	if _, err := os.Stat(filepath.Join(a, "1", "values.yaml")); err == nil {
		t.Fatalf("unexpected values.yaml at %s", filepath.Join(a, "1"))
	}
	if _, err := os.Stat(filepath.Join(a, "2", "values.yaml")); err == nil {
		t.Fatalf("unexpected values.yaml at %s", filepath.Join(a, "2"))
	}
}

func TestExtractCommonRecursive_ChildAndGrandchild_CommonAtParent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	oneX := filepath.Join(a, "1", "X")
	two := filepath.Join(a, "2")
	mustMkdirAll(t, oneX)
	mustMkdirAll(t, two)

	p1 := filepath.Join(oneX, "values.yaml")
	p2 := filepath.Join(two, "values.yaml")
	mustWriteFile(t, p1, []byte(`foo:
  bar:
    something: [1,2,3]
    other: true
`))
	mustWriteFile(t, p2, []byte(`foo:
  bar:
    else: [1,2,3]
    other: true
`))

	created, err := ExtractCommonRecursive(dir)
	if err != nil {
		t.Fatalf("ExtractCommonRecursive error: %v", err)
	}
	// Expect /a/values.yaml only
	expect := []string{filepath.Join(a, "values.yaml")}
	if !reflect.DeepEqual(expect, created) {
		t.Fatalf("unexpected created: %v", created)
	}
	assertYAMLEqual(t, []byte(`foo:
  bar:
    other: true
`), mustReadFile(t, filepath.Join(a, "values.yaml")))
	assertYAMLEqual(t, []byte(`foo:
  bar:
    something:
    - 1
    - 2
    - 3
`), mustReadFile(t, p1))
	assertYAMLEqual(t, []byte(`foo:
  bar:
    else:
    - 1
    - 2
    - 3
`), mustReadFile(t, p2))
	// Ensure no intermediate values.yaml at /a/1
	if _, err := os.Stat(filepath.Join(a, "1", "values.yaml")); err == nil {
		t.Fatalf("unexpected values.yaml at %s", filepath.Join(a, "1"))
	}
}

func TestExtractCommonRecursive_MixedDepthThreeDescendants(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	oneX := filepath.Join(a, "1", "X")
	two := filepath.Join(a, "2")
	threeZ := filepath.Join(a, "3", "Z")
	for _, d := range []string{oneX, two, threeZ} {
		mustMkdirAll(t, d)
	}
	p1 := filepath.Join(oneX, "values.yaml")
	p2 := filepath.Join(two, "values.yaml")
	p3 := filepath.Join(threeZ, "values.yaml")

	// Common part across all three: meta.env: prod; differing unique keys
	mustWriteFile(t, p1, []byte(`meta:
  env: prod
svc:
  a: 1
`))
	mustWriteFile(t, p2, []byte(`meta:
  env: prod
cfg:
  b: 2
`))
	mustWriteFile(t, p3, []byte(`meta:
  env: prod
feat:
  c: 3
`))

	created, err := ExtractCommonRecursive(dir)
	if err != nil {
		t.Fatalf("ExtractCommonRecursive error: %v", err)
	}
	// Expect /a/values.yaml only
	expect := []string{filepath.Join(a, "values.yaml")}
	if !reflect.DeepEqual(expect, created) {
		t.Fatalf("unexpected created: %v", created)
	}
	assertYAMLEqual(t, []byte(`meta:
  env: prod
`), mustReadFile(t, filepath.Join(a, "values.yaml")))
	assertYAMLEqual(t, []byte(`svc:
  a: 1
`), mustReadFile(t, p1))
	assertYAMLEqual(t, []byte(`cfg:
  b: 2
`), mustReadFile(t, p2))
	assertYAMLEqual(t, []byte(`feat:
  c: 3
`), mustReadFile(t, p3))
	// Ensure no intermediate values.yaml at /a/1 or /a/3
	if _, err := os.Stat(filepath.Join(a, "1", "values.yaml")); err == nil {
		t.Fatalf("unexpected values.yaml at %s", filepath.Join(a, "1"))
	}
	if _, err := os.Stat(filepath.Join(a, "3", "values.yaml")); err == nil {
		t.Fatalf("unexpected values.yaml at %s", filepath.Join(a, "3"))
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
