package values

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	yamllib "github.com/inercia/go-values-yaml/pkg/yaml"
	"github.com/psanford/memfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractCommon tests the basic ExtractCommon functionality
func TestExtractCommon(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		y1           []byte
		y2           []byte
		wantCommon   []byte
		wantUpdated1 []byte
		wantUpdated2 []byte
		wantErr      bool
	}{
		{
			name: "simple common extraction",
			y1: []byte(`foo:
  bar:
    something: [1,2,3]
    other: true`),
			y2: []byte(`foo:
  bar:
    else: [1,2,3]
    other: true`),
			wantCommon: []byte(`foo:
  bar:
    other: true`),
			wantUpdated1: []byte(`foo:
  bar:
    something:
    - 1
    - 2
    - 3`),
			wantUpdated2: []byte(`foo:
  bar:
    else:
    - 1
    - 2
    - 3`),
		},
		{
			name:    "no common values",
			y1:      []byte(`a: 1`),
			y2:      []byte(`b: 2`),
			wantErr: true,
		},
		{
			name: "deep nested maps",
			y1: []byte(`root:
  x:
    shared:
      deep:
        inner:
          alpha: 1
          beta: 2
  svc:
    image: a:v1`),
			y2: []byte(`root:
  svc:
    image: b:v1
  x:
    shared:
      deep:
        inner:
          beta: 2
          alpha: 1`),
			wantCommon: []byte(`root:
  x:
    shared:
      deep:
        inner:
          alpha: 1
          beta: 2`),
			wantUpdated1: []byte(`root:
  svc:
    image: a:v1`),
			wantUpdated2: []byte(`root:
  svc:
    image: b:v1`),
		},
		{
			name:    "empty files",
			y1:      []byte(""),
			y2:      []byte(""),
			wantErr: true,
		},
		{
			name: "unicode characters",
			y1: []byte(`app:
  name: "测试应用"
  emoji: "🚀"
  unicode: "ñáéíóú"`),
			y2: []byte(`app:
  name: "测试应用"
  emoji: "🚀"
  unicode: "ñáéíóú"
other: different`),
			wantCommon: []byte(`app:
  emoji: 🚀
  name: 测试应用
  unicode: ñáéíóú`),
			wantUpdated1: []byte(`{}`),
			wantUpdated2: []byte(`other: different`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, dirs := setupTempDirs(t, "a/b/x", "a/b/y")
			paths := setupValuesFiles(t, dirs, [][]byte{tc.y1, tc.y2})

			commonPath, err := ExtractCommon(paths[0], paths[1])

			if tc.wantErr {
				assert.Error(t, err)
				if err != nil {
					assert.ErrorIs(t, err, ErrNoCommon)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, filepath.Join(filepath.Dir(dirs[0]), "values.yaml"), commonPath)

			if tc.wantCommon != nil {
				assertYAMLEqual(t, tc.wantCommon, mustReadFile(t, commonPath))
			}
			if tc.wantUpdated1 != nil {
				assertYAMLEqual(t, tc.wantUpdated1, mustReadFile(t, paths[0]))
			}
			if tc.wantUpdated2 != nil {
				assertYAMLEqual(t, tc.wantUpdated2, mustReadFile(t, paths[1]))
			}

			// Validate merge property: merge(common, updated) == original
			if !tc.wantErr && tc.wantCommon != nil {
				common := mustReadFile(t, commonPath)
				updated1 := mustReadFile(t, paths[0])
				updated2 := mustReadFile(t, paths[1])
				validateMergeProperty(t, tc.y1, common, updated1)
				validateMergeProperty(t, tc.y2, common, updated2)
			}
		})
	}
}

// TestExtractCommonN tests extraction with multiple files
func TestExtractCommonN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputs      [][]byte
		wantCommon  []byte
		wantUpdates [][]byte
		wantErr     bool
	}{
		{
			name: "three files with common subset",
			inputs: [][]byte{
				[]byte(`foo:
  bar:
    a: 1
    same: true`),
				[]byte(`foo:
  bar:
    b: 2
    same: true`),
				[]byte(`foo:
  bar:
    c: 3
    same: true`),
			},
			wantCommon: []byte(`foo:
  bar:
    same: true`),
			wantUpdates: [][]byte{
				[]byte(`foo:
  bar:
    a: 1`),
				[]byte(`foo:
  bar:
    b: 2`),
				[]byte(`foo:
  bar:
    c: 3`),
			},
		},
		{
			name: "five microservices",
			inputs: [][]byte{
				[]byte(`metadata:
  org: company
  env: prod
app:
  name: auth
  port: 8080`),
				[]byte(`metadata:
  org: company
  env: prod
app:
  name: users
  port: 8081`),
				[]byte(`metadata:
  org: company
  env: prod
app:
  name: orders
  port: 8082`),
				[]byte(`metadata:
  org: company
  env: prod
app:
  name: payments
  port: 8083`),
				[]byte(`metadata:
  org: company
  env: prod
app:
  name: notifications
  port: 8084`),
			},
			wantCommon: []byte(`metadata:
  env: prod
  org: company`),
		},
		{
			name:    "single file should error",
			inputs:  [][]byte{[]byte("key: value")},
			wantErr: true,
		},
		{
			name:    "zero files should error",
			inputs:  [][]byte{},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.inputs) == 0 {
				_, err := ExtractCommonN([]string{})
				assert.Error(t, err)
				return
			}

			dirs := make([]string, len(tc.inputs))
			for i := range tc.inputs {
				dirs[i] = filepath.Join("apps", string('a'+rune(i)))
			}
			_, fullDirs := setupTempDirs(t, dirs...)
			paths := setupValuesFiles(t, fullDirs, tc.inputs)

			commonPath, err := ExtractCommonN(paths)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tc.wantCommon != nil {
				assertYAMLEqual(t, tc.wantCommon, mustReadFile(t, commonPath))
			}

			if tc.wantUpdates != nil {
				for i, p := range paths {
					assertYAMLEqual(t, tc.wantUpdates[i], mustReadFile(t, p))
				}
			}

			// Validate merge property for all files
			if tc.wantCommon != nil {
				common := mustReadFile(t, commonPath)
				for i, originalContent := range tc.inputs {
					updated := mustReadFile(t, paths[i])
					validateMergeProperty(t, originalContent, common, updated)
				}
			}
		})
	}
}

// TestExtractCommonRecursive tests recursive extraction
func TestExtractCommonRecursive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setup         func(t *testing.T) (string, map[string][]byte)
		expectCreated []string // relative paths
		validateFunc  func(t *testing.T, root string)
	}{
		{
			name: "single parent group",
			setup: func(t *testing.T) (string, map[string][]byte) {
				files := map[string][]byte{
					"apps/svc-a/values.yaml": []byte(`foo:
  bar:
    a: 1
    common: yes`),
					"apps/svc-b/values.yaml": []byte(`foo:
  bar:
    b: 2
    common: yes`),
				}
				return "apps", files
			},
			expectCreated: []string{"values.yaml"},
			validateFunc: func(t *testing.T, root string) {
				assertYAMLEqual(t, []byte(`foo:
  bar:
    common: yes`), mustReadFile(t, filepath.Join(root, "values.yaml")))
			},
		},
		{
			name: "multiple groups at different levels",
			setup: func(t *testing.T) (string, map[string][]byte) {
				files := map[string][]byte{
					"env/prod/app1/values.yaml": []byte(`cfg:
  image: v1
  replicas: 2`),
					"env/prod/app2/values.yaml": []byte(`cfg:
  image: v1
  replicas: 3`),
					"env/staging/app3/values.yaml": []byte(`cfg:
  image: v2
  replicas: 5`),
					"env/staging/app4/values.yaml": []byte(`cfg:
  image: v2
  replicas: 1`),
				}
				return "env", files
			},
			expectCreated: []string{"prod/values.yaml", "staging/values.yaml"},
			validateFunc: func(t *testing.T, root string) {
				assertYAMLEqual(t, []byte(`cfg:
  image: v1`), mustReadFile(t, filepath.Join(root, "prod/values.yaml")))
				assertYAMLEqual(t, []byte(`cfg:
  image: v2`), mustReadFile(t, filepath.Join(root, "staging/values.yaml")))
			},
		},
		{
			name: "two-level propagation",
			setup: func(t *testing.T) (string, map[string][]byte) {
				files := map[string][]byte{
					"a/b/x/values.yaml": []byte(`svc:
  team: core
  image: v1
  replicas: 1`),
					"a/b/y/values.yaml": []byte(`svc:
  team: core
  image: v1
  replicas: 2`),
					"a/c/u/values.yaml": []byte(`svc:
  team: core
  image: v2
  replicas: 3`),
					"a/c/v/values.yaml": []byte(`svc:
  team: core
  image: v2
  replicas: 4`),
				}
				return "a", files
			},
			expectCreated: []string{"b/values.yaml", "c/values.yaml", "values.yaml"},
			validateFunc: func(t *testing.T, root string) {
				// Top level should have team: core
				assertYAMLEqual(t, []byte(`svc:
  team: core`), mustReadFile(t, filepath.Join(root, "values.yaml")))
				// Mid level should have images
				assertYAMLEqual(t, []byte(`svc:
  image: v1`), mustReadFile(t, filepath.Join(root, "b/values.yaml")))
				assertYAMLEqual(t, []byte(`svc:
  image: v2`), mustReadFile(t, filepath.Join(root, "c/values.yaml")))
			},
		},
		{
			name: "no common values",
			setup: func(t *testing.T) (string, map[string][]byte) {
				files := map[string][]byte{
					"group/x/values.yaml": []byte("a: 1"),
					"group/y/values.yaml": []byte("b: 2"),
				}
				return "group", files
			},
			expectCreated: []string{},
		},
		{
			name: "grandchildren extraction",
			setup: func(t *testing.T) (string, map[string][]byte) {
				files := map[string][]byte{
					"a/1/X/values.yaml": []byte(`foo:
  bar:
    something: [1,2,3]
    other: true`),
					"a/2/Y/values.yaml": []byte(`foo:
  bar:
    else: [1,2,3]
    other: true`),
				}
				return "a", files
			},
			expectCreated: []string{"values.yaml"},
			validateFunc: func(t *testing.T, root string) {
				assertYAMLEqual(t, []byte(`foo:
  bar:
    other: true`), mustReadFile(t, filepath.Join(root, "values.yaml")))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			rootName, files := tc.setup(t)
			root := filepath.Join(dir, rootName)

			// Create all files
			for path, content := range files {
				fullPath := filepath.Join(dir, path)
				mustWriteFile(t, fullPath, content)
			}

			created, err := ExtractCommonRecursive(root)
			require.NoError(t, err)

			// Convert created to relative paths
			createdRel := []string{}
			for _, p := range created {
				rel, err := filepath.Rel(root, p)
				require.NoError(t, err)
				createdRel = append(createdRel, rel)
			}
			sort.Strings(createdRel)
			sort.Strings(tc.expectCreated)

			assert.Equal(t, tc.expectCreated, createdRel, "unexpected created files")

			if tc.validateFunc != nil {
				tc.validateFunc(t, root)
			}
		})
	}
}

// TestExtractCommonValidation tests various validation scenarios
func TestExtractCommonValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) (string, string)
		wantErr bool
		errMsg  string
	}{
		{
			name: "non-sibling files",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				p1 := filepath.Join(dir, "a/x/values.yaml")
				p2 := filepath.Join(dir, "b/y/values.yaml")
				mustWriteFile(t, p1, []byte("a: 1"))
				mustWriteFile(t, p2, []byte("a: 1"))
				return p1, p2
			},
			wantErr: true,
			errMsg:  "same parent directory",
		},
		{
			name: "wrong filename",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				p1 := filepath.Join(dir, "a/x/config.yaml")
				p2 := filepath.Join(dir, "a/y/values.yaml")
				mustWriteFile(t, p1, []byte("a: 1"))
				mustWriteFile(t, p2, []byte("a: 1"))
				return p1, p2
			},
			wantErr: true,
			errMsg:  "must be named values.yaml",
		},
		{
			name: "malformed YAML",
			setup: func(t *testing.T) (string, string) {
				_, dirs := setupTempDirs(t, "a/b/x", "a/b/y")
				p1 := filepath.Join(dirs[0], "values.yaml")
				p2 := filepath.Join(dirs[1], "values.yaml")
				mustWriteFile(t, p1, []byte("invalid: ["))
				mustWriteFile(t, p2, []byte("valid: true"))
				return p1, p2
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p1, p2 := tc.setup(t)
			_, err := ExtractCommon(p1, p2)

			if tc.wantErr {
				assert.Error(t, err)
				if tc.errMsg != "" {
					assert.Contains(t, err.Error(), tc.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestExtractCommonWithMemFS tests extraction using in-memory filesystem
func TestExtractCommonWithMemFS(t *testing.T) {
	t.Parallel()

	mfs := memfs.New()
	ops := memfsOps{fsys: mfs}

	// Setup test structure
	writeMemFile(t, mfs, "apps/svc-a/values.yaml", []byte(`global:
  company: acme
  env: prod
service:
  name: svc-a
  port: 8080`))

	writeMemFile(t, mfs, "apps/svc-b/values.yaml", []byte(`global:
  company: acme
  env: prod
service:
  name: svc-b
  port: 8081`))

	commonPath, err := ExtractCommon(
		"apps/svc-a/values.yaml",
		"apps/svc-b/values.yaml",
		WithFileOps(ops),
	)

	require.NoError(t, err)
	assert.Equal(t, "apps/values.yaml", commonPath)

	// Verify common content
	common := readMemFile(t, mfs, commonPath)
	assertYAMLEqual(t, []byte(`global:
  company: acme
  env: prod`), common)

	// Verify updated files
	updated1 := readMemFile(t, mfs, "apps/svc-a/values.yaml")
	assertYAMLEqual(t, []byte(`service:
  name: svc-a
  port: 8080`), updated1)

	updated2 := readMemFile(t, mfs, "apps/svc-b/values.yaml")
	assertYAMLEqual(t, []byte(`service:
  name: svc-b
  port: 8081`), updated2)
}

// TestMergePropertyHolds validates that merge(common, updated) == original for all operations
func TestMergePropertyHolds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		y1, y2 []byte
	}{
		{
			name: "simple nested structure",
			y1: []byte(`app:
  name: service-a
  config:
    env: prod
    replicas: 2
    shared: true`),
			y2: []byte(`app:
  name: service-b
  config:
    env: prod
    replicas: 1
    shared: true`),
		},
		{
			name: "complex with arrays",
			y1: []byte(`metadata:
  team: platform
  labels:
    env: production
    tier: backend
service:
  ports:
    - name: http
      port: 80
    - name: grpc
      port: 9000
  env:
    - name: DB_HOST
      value: prod-db
    - name: LOG_LEVEL
      value: info`),
			y2: []byte(`metadata:
  team: platform
  labels:
    env: production
    tier: frontend
service:
  ports:
    - name: http
      port: 80
    - name: grpc
      port: 9000
  env:
    - name: DB_HOST
      value: prod-db
    - name: LOG_LEVEL
      value: debug`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, dirs := setupTempDirs(t, "a/b/x", "a/b/y")
			paths := setupValuesFiles(t, dirs, [][]byte{tc.y1, tc.y2})

			commonPath, err := ExtractCommon(paths[0], paths[1])
			require.NoError(t, err)

			// Read the results
			common := mustReadFile(t, commonPath)
			updated1 := mustReadFile(t, paths[0])
			updated2 := mustReadFile(t, paths[1])

			// Validate merge property
			reconstructed1, err := yamllib.MergeYAML(common, updated1)
			require.NoError(t, err)
			assertYAMLEqual(t, tc.y1, reconstructed1)

			reconstructed2, err := yamllib.MergeYAML(common, updated2)
			require.NoError(t, err)
			assertYAMLEqual(t, tc.y2, reconstructed2)
		})
	}
}

// TestExtractCommonPermissions tests behavior with file permission issues
func TestExtractCommonPermissions(t *testing.T) {
	// Skip on Windows as chmod doesn't work the same way
	if os.Getenv("OS") == "Windows_NT" {
		t.Skip("Skipping permission test on Windows")
	}

	t.Run("read-only parent directory", func(t *testing.T) {
		dir := t.TempDir()
		parent := filepath.Join(dir, "parent")
		x := filepath.Join(parent, "x")
		y := filepath.Join(parent, "y")
		mustMkdirAll(t, x)
		mustMkdirAll(t, y)

		p1 := filepath.Join(x, "values.yaml")
		p2 := filepath.Join(y, "values.yaml")
		mustWriteFile(t, p1, []byte("shared: value\nunique1: val1"))
		mustWriteFile(t, p2, []byte("shared: value\nunique2: val2"))

		// Make parent read-only
		cleanup := makeReadOnly(t, parent)
		defer cleanup()

		_, err := ExtractCommon(p1, p2)
		assert.Error(t, err, "expected error for read-only parent")

		// Ensure files are unchanged
		assertYAMLEqual(t, []byte("shared: value\nunique1: val1"), mustReadFile(t, p1))
		assertYAMLEqual(t, []byte("shared: value\nunique2: val2"), mustReadFile(t, p2))
	})
}
