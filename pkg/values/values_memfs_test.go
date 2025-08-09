package values

import (
	"io/fs"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/psanford/memfs"
)

// memfsOps implements fileOps on top of github.com/psanford/memfs
// for use in tests.
type memfsOps struct{ fsys *memfs.FS }

func (m memfsOps) Stat(name string) (fs.FileInfo, error)        { return fs.Stat(m.fsys, name) }
func (m memfsOps) ReadFile(name string) ([]byte, error)         { return fs.ReadFile(m.fsys, name) }
func (m memfsOps) WalkDir(root string, fn fs.WalkDirFunc) error { return fs.WalkDir(m.fsys, root, fn) }
func (m memfsOps) WriteFileAtomic(path string, data []byte, perm fs.FileMode) error {
	if err := m.fsys.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return m.fsys.WriteFile(path, data, perm)
}

func TestExtractCommonRecursive_MemFS_DeepHierarchyMultiLevel(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "recursive extraction across prod/staging/apps/tools -> multi-level commons",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mfs := memfs.New()
			ops := memfsOps{fsys: mfs}
			root := "root"

			paths := []string{
				"root/org/prod/apps/api",
				"root/org/prod/apps/web",
				"root/org/prod/tools/monitor",
				"root/org/prod/tools/backup",
				"root/org/staging/apps/api",
				"root/org/staging/apps/web",
				"root/org/staging/tools/monitor",
				"root/org/staging/tools/backup",
			}
			for _, p := range paths {
				if err := mfs.MkdirAll(p, 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
			}

			// prod/apps
			writeY(t, mfs, "root/org/prod/apps/api/values.yaml", []byte(`meta:
  company: acme
  policy:
    logs: true
svc:
  team: app
  image: api:v1
  port: 80
  replicas: 2
`))
			writeY(t, mfs, "root/org/prod/apps/web/values.yaml", []byte(`meta:
  company: acme
  policy:
    logs: true
svc:
  team: app
  image: web:v1
  port: 80
  replicas: 3
`))
			// prod/tools
			writeY(t, mfs, "root/org/prod/tools/monitor/values.yaml", []byte(`meta:
  company: acme
  policy:
    logs: true
ops:
  team: ops
  monitoring: true
  endpoints:
    - metrics
`))
			writeY(t, mfs, "root/org/prod/tools/backup/values.yaml", []byte(`meta:
  company: acme
  policy:
    logs: true
ops:
  team: ops
  monitoring: true
  endpoints:
    - backups
`))
			// staging/apps
			writeY(t, mfs, "root/org/staging/apps/api/values.yaml", []byte(`meta:
  company: acme
  policy:
    logs: true
svc:
  team: app
  image: api:v2
  port: 80
  replicas: 1
`))
			writeY(t, mfs, "root/org/staging/apps/web/values.yaml", []byte(`meta:
  company: acme
  policy:
    logs: true
svc:
  team: app
  image: web:v2
  port: 80
  replicas: 1
`))
			// staging/tools
			writeY(t, mfs, "root/org/staging/tools/monitor/values.yaml", []byte(`meta:
  company: acme
  policy:
    logs: true
ops:
  team: ops
  monitoring: true
  endpoints:
    - metrics
`))
			writeY(t, mfs, "root/org/staging/tools/backup/values.yaml", []byte(`meta:
  company: acme
  policy:
    logs: true
ops:
  team: ops
  monitoring: true
  endpoints:
    - backups
`))

			created, err := ExtractCommonRecursive(filepath.Join(root, "org"), WithFileOps(ops))
			if err != nil {
				t.Fatalf("ExtractCommonRecursive error: %v", err)
			}
			expect := []string{
				"root/org/prod/apps/values.yaml",
				"root/org/prod/tools/values.yaml",
				"root/org/staging/apps/values.yaml",
				"root/org/staging/tools/values.yaml",
				"root/org/prod/values.yaml",
				"root/org/staging/values.yaml",
				"root/org/values.yaml",
			}
			sort.Strings(expect)
			if !reflect.DeepEqual(expect, created) {
				t.Fatalf("unexpected created list:\nexpect: %v\n   got: %v", expect, created)
			}

			assertYAMLEqual(t, []byte(`svc:
  team: app
  port: 80
`), readY(t, mfs, "root/org/prod/apps/values.yaml"))
			assertYAMLEqual(t, []byte(`svc:
  image: api:v1
  replicas: 2
`), readY(t, mfs, "root/org/prod/apps/api/values.yaml"))
			assertYAMLEqual(t, []byte(`svc:
  image: web:v1
  replicas: 3
`), readY(t, mfs, "root/org/prod/apps/web/values.yaml"))

			assertYAMLEqual(t, []byte(`ops:
  team: ops
  monitoring: true
`), readY(t, mfs, "root/org/prod/tools/values.yaml"))
			assertYAMLEqual(t, []byte(`{}
`), readY(t, mfs, "root/org/prod/values.yaml"))
			assertYAMLEqual(t, []byte(`{}
`), readY(t, mfs, "root/org/staging/values.yaml"))
			assertYAMLEqual(t, []byte(`meta:
  company: acme
  policy:
    logs: true
`), readY(t, mfs, "root/org/values.yaml"))
		})
	}
}

func TestExtractCommonN_MemFS_EqualListsOption(t *testing.T) {
	tests := []struct {
		name           string
		root           string
		lists          [][]string
		disableEqual   bool
		wantCommonPath string
		wantCommonYAML []byte
		wantErr        error
	}{
		{
			name:           "equal lists go to common by default",
			root:           "root/apps",
			lists:          [][]string{{"x", "y"}, {"x", "y"}, {"x", "y"}},
			disableEqual:   false,
			wantCommonPath: "root/apps/values.yaml",
			wantCommonYAML: []byte("cfg:\n  list:\n    - x\n    - y\n"),
			wantErr:        nil,
		},
		{
			name:           "equal lists excluded -> ErrNoCommon and no common file",
			root:           "grp",
			lists:          [][]string{{"1", "2", "3"}, {"1", "2", "3"}},
			disableEqual:   true,
			wantCommonPath: "",
			wantCommonYAML: nil,
			wantErr:        ErrNoCommon,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mfs := memfs.New()
			ops := memfsOps{fsys: mfs}
			// setup paths
			var paths []string
			for i := range tc.lists {
				p := filepath.Join(tc.root, string('a'+rune(i)), "values.yaml")
				_ = mfs.MkdirAll(filepath.Dir(p), 0o755)
				paths = append(paths, p)
				// write YAML
				if tc.root == "grp" {
					// special case to match previous test expectations
					writeY(t, mfs, p, []byte("cfg:\n  list: ["+tc.lists[i][0]+","+tc.lists[i][1]+","+tc.lists[i][2]+"]\n"))
				} else {
					writeY(t, mfs, p, []byte("cfg:\n  list:\n    - "+tc.lists[i][0]+"\n    - "+tc.lists[i][1]+"\n"))
				}
			}
			// run
			var cp string
			var err error
			if tc.disableEqual {
				cp, err = ExtractCommonN(paths, WithFileOps(ops), WithIncludeEqualListsInCommon(false))
			} else {
				cp, err = ExtractCommonN(paths, WithFileOps(ops))
			}
			if tc.wantErr != nil {
				if err == nil || err != tc.wantErr {
					t.Fatalf("expected %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ExtractCommonN error: %v", err)
			}
			if tc.wantCommonPath != "" && cp != tc.wantCommonPath {
				t.Fatalf("unexpected common path: %s", cp)
			}
			if tc.wantCommonYAML != nil {
				assertYAMLEqual(t, tc.wantCommonYAML, readY(t, mfs, cp))
			}
		})
	}
}

func TestExtractCommon_MemFS_NoCommon_NoChanges(t *testing.T) {
	mfs := memfs.New()
	ops := memfsOps{fsys: mfs}
	_ = mfs.MkdirAll("grp/x", 0o755)
	_ = mfs.MkdirAll("grp/y", 0o755)
	writeY(t, mfs, "grp/x/values.yaml", []byte("a: 1\n"))
	writeY(t, mfs, "grp/y/values.yaml", []byte("b: 2\n"))
	_, err := ExtractCommon("grp/x/values.yaml", "grp/y/values.yaml", WithFileOps(ops))
	if err == nil {
		t.Fatalf("expected ErrNoCommon, got nil")
	}
	if err != ErrNoCommon {
		t.Fatalf("expected ErrNoCommon, got %v", err)
	}
	assertYAMLEqual(t, []byte("a: 1\n"), readY(t, mfs, "grp/x/values.yaml"))
	assertYAMLEqual(t, []byte("b: 2\n"), readY(t, mfs, "grp/y/values.yaml"))
}

// Helpers for memfs tests
func writeY(t *testing.T, mfs *memfs.FS, path string, data []byte) {
	t.Helper()
	if err := mfs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := mfs.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func readY(t *testing.T, mfs *memfs.FS, path string) []byte {
	t.Helper()
	b, err := fs.ReadFile(mfs, path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return b
}
