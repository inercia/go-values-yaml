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

func (m memfsOps) Stat(name string) (fs.FileInfo, error)                 { return fs.Stat(m.fsys, name) }
func (m memfsOps) ReadFile(name string) ([]byte, error)                  { return fs.ReadFile(m.fsys, name) }
func (m memfsOps) WalkDir(root string, fn fs.WalkDirFunc) error          { return fs.WalkDir(m.fsys, root, fn) }
func (m memfsOps) WriteFileAtomic(path string, data []byte, perm fs.FileMode) error {
	if err := m.fsys.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return m.fsys.WriteFile(path, data, perm)
}

func TestExtractCommonRecursive_MemFS_DeepHierarchyMultiLevel(t *testing.T) {
	mfs := memfs.New()
	ops := memfsOps{fsys: mfs}
	root := "root"

	// Build a deep tree with two top-level environments, each with apps and tools groups
	// Each group has two services with some shared config
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

	// prod/apps shared: service.port 80, team app; differing replicas
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

	// prod/tools shared: ops team, monitoring on
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

	// staging/apps shared: same as prod/apps but different images/replicas
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

	// staging/tools shared: ops team, monitoring on
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

	// Now run recursive extraction on memfs
	created, err := ExtractCommonRecursive(filepath.Join(root, "org"), WithFileOps(ops))
	if err != nil {
		t.Fatalf("ExtractCommonRecursive error: %v", err)
	}

	// Expect parents created at each apps/tools group under prod and staging, and at prod and staging themselves,
	// and finally at org (top-level) because meta.company/policy.logs are common across prod and staging after consolidation.
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

	// Validate group-level commons for prod/apps (meta moved up to prod/org)
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

	// Validate group-level commons for prod/tools (meta moved up to prod/org)
	assertYAMLEqual(t, []byte(`ops:
  team: ops
  monitoring: true
`), readY(t, mfs, "root/org/prod/tools/values.yaml"))

	// Validate prod-level after org-level consolidation: becomes empty
	assertYAMLEqual(t, []byte(`{}
`), readY(t, mfs, "root/org/prod/values.yaml"))

	// Validate staging-level after org-level consolidation: becomes empty
	assertYAMLEqual(t, []byte(`{}
`), readY(t, mfs, "root/org/staging/values.yaml"))

	// Validate top-level org common across prod and staging (meta)
	assertYAMLEqual(t, []byte(`meta:
  company: acme
  policy:
    logs: true
`), readY(t, mfs, "root/org/values.yaml"))
}

func TestExtractCommonN_MemFS_EqualListsOption(t *testing.T) {
	mfs := memfs.New()
	ops := memfsOps{fsys: mfs}
	root := "root/apps"
	_ = mfs.MkdirAll("root/apps/a", 0o755)
	_ = mfs.MkdirAll("root/apps/b", 0o755)
	_ = mfs.MkdirAll("root/apps/c", 0o755)

	writeY(t, mfs, "root/apps/a/values.yaml", []byte(`cfg:
  list:
    - x
    - y
`))
	writeY(t, mfs, "root/apps/b/values.yaml", []byte(`cfg:
  list:
    - x
    - y
`))
	writeY(t, mfs, "root/apps/c/values.yaml", []byte(`cfg:
  list:
    - x
    - y
`))

	paths := []string{
		"root/apps/a/values.yaml",
		"root/apps/b/values.yaml",
		"root/apps/c/values.yaml",
	}
	// By default equal lists go to common
	cp, err := ExtractCommonN(paths, WithFileOps(ops))
	if err != nil {
		t.Fatalf("ExtractCommonN error: %v", err)
	}
	if cp != filepath.Join(root, "values.yaml") {
		t.Fatalf("unexpected common path: %s", cp)
	}
	assertYAMLEqual(t, []byte(`cfg:
  list:
    - x
    - y
`), readY(t, mfs, cp))

	// When disabled, the list remains in each file and no common file should be created
	mfs2 := memfs.New()
	ops2 := memfsOps{fsys: mfs2}
	_ = mfs2.MkdirAll("grp/a", 0o755)
	_ = mfs2.MkdirAll("grp/b", 0o755)
	writeY(t, mfs2, "grp/a/values.yaml", []byte(`cfg:
  list: [1,2,3]
`))
	writeY(t, mfs2, "grp/b/values.yaml", []byte(`cfg:
  list: [1,2,3]
`))
	_, err = ExtractCommonN([]string{"grp/a/values.yaml", "grp/b/values.yaml"}, WithFileOps(ops2), WithIncludeEqualListsInCommon(false))
	if err == nil {
		t.Fatalf("expected ErrNoCommon when equal lists excluded, got nil")
	}
	if err != ErrNoCommon {
		t.Fatalf("expected ErrNoCommon, got %v", err)
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
