package values

import (
	"path/filepath"
	"testing"

	yamllib "github.com/inercia/go-values-yaml/pkg/yaml"
)

// TestMergeProperty validates that the fundamental property holds:
// merge(common, updated) == original for all extraction operations.
// This is critical for Helm compatibility.
func TestMergeProperty_ExtractCommon(t *testing.T) {
	tests := []struct {
		name string
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
			name: "complex nested with arrays",
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
		{
			name: "mixed types and different values",
			y1: []byte(`config:
  database:
    host: localhost
    port: 5432
    ssl: true
  features:
    enabled: true
    count: 42
  app:
    name: "test-app-1"
    debug: false`),
			y2: []byte(`config:
  database:
    host: localhost
    port: 5432
    ssl: true
  features:
    enabled: true
    count: 10
  app:
    name: "test-app-2"
    debug: true`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, dirs := setupTempDirs(t, "a/b/x", "a/b/y")
			paths := setupValuesFiles(t, dirs, [][]byte{tc.y1, tc.y2})
			
			commonPath, err := ExtractCommon(paths[0], paths[1])
			if err != nil {
				t.Fatalf("ExtractCommon failed: %v", err)
			}

			// Read the results
			common := mustReadFile(t, commonPath)
			updated1 := mustReadFile(t, paths[0])
			updated2 := mustReadFile(t, paths[1])

			// Validate merge property for both files
			validateMergeProperty(t, tc.y1, common, updated1)
			validateMergeProperty(t, tc.y2, common, updated2)
		})
	}
}

func TestMergeProperty_ExtractCommonN(t *testing.T) {
	tests := []struct {
		name string
		inputs [][]byte
	}{
		{
			name: "three services with shared config",
			inputs: [][]byte{
				[]byte(`global:
  registry: docker.io
  pullPolicy: Always
service:
  type: ClusterIP
  name: api
  replicas: 3`),
				[]byte(`global:
  registry: docker.io
  pullPolicy: Always
service:
  type: ClusterIP
  name: web
  replicas: 2`),
				[]byte(`global:
  registry: docker.io
  pullPolicy: Always
service:
  type: ClusterIP
  name: worker
  replicas: 1`),
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
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dirs := make([]string, len(tc.inputs))
			for i := range tc.inputs {
				dirs[i] = filepath.Join("apps", string('a'+rune(i)))
			}
			_, fullDirs := setupTempDirs(t, dirs...)
			paths := setupValuesFiles(t, fullDirs, tc.inputs)

			commonPath, err := ExtractCommonN(paths)
			if err != nil {
				t.Fatalf("ExtractCommonN failed: %v", err)
			}

			// Read common file
			common := mustReadFile(t, commonPath)

			// Validate merge property for all files
			for i, originalContent := range tc.inputs {
				updated := mustReadFile(t, paths[i])
				validateMergeProperty(t, originalContent, common, updated)
			}
		})
	}
}

func TestMergeProperty_ExtractCommonRecursive(t *testing.T) {
	t.Run("multi-level hierarchy preserves merge property", func(t *testing.T) {
		_, dirs := setupTempDirs(t, 
			"env/prod/apps/api",
			"env/prod/apps/web", 
			"env/prod/tools/monitor",
			"env/staging/apps/api",
			"env/staging/apps/web",
		)
		
		originals := [][]byte{
			[]byte(`global:
  org: company
  env: prod
app:
  team: backend
  service: api
  port: 8080`),
			[]byte(`global:
  org: company
  env: prod
app:
  team: backend
  service: web
  port: 3000`),
			[]byte(`global:
  org: company
  env: prod
ops:
  team: sre
  service: monitor
  port: 9090`),
			[]byte(`global:
  org: company
  env: staging
app:
  team: backend
  service: api
  port: 8080`),
			[]byte(`global:
  org: company
  env: staging
app:
  team: backend
  service: web
  port: 3000`),
		}

		paths := setupValuesFiles(t, dirs, originals)
		
		// Store original content for validation
		originalFiles := make([][]byte, len(paths))
		for i, path := range paths {
			originalFiles[i] = mustReadFile(t, path)
		}

		rootDir := filepath.Dir(filepath.Dir(dirs[0]))
		created, err := ExtractCommonRecursive(rootDir)
		if err != nil {
			t.Fatalf("ExtractCommonRecursive failed: %v", err)
		}

		if len(created) == 0 {
			t.Fatalf("expected some common files to be created")
		}

		// For each original file, validate that we can reconstruct it
		// by merging all applicable common files with the remainder
		for i, path := range paths {
			// Find all applicable common files (ancestors)
			var commonFiles [][]byte
			dir := filepath.Dir(path)
			for {
				commonPath := filepath.Join(dir, "values.yaml")
				// Check if this is one of the created common files
				isCommonFile := false
				for _, cp := range created {
					if cp == commonPath {
						isCommonFile = true
						break
					}
				}
				if isCommonFile {
					commonFiles = append([][]byte{mustReadFile(t, commonPath)}, commonFiles...)
				}
				parent := filepath.Dir(dir)
				if parent == dir || parent == rootDir {
					break
				}
				dir = parent
			}

			// If we found common files, validate the merge property
			if len(commonFiles) > 0 {
				updated := mustReadFile(t, path)
				// Merge all common files with the updated content
				result := updated
				for _, common := range commonFiles {
					result, err = yamllib.MergeYAML(common, result)
					if err != nil {
						t.Fatalf("merge failed for path %s: %v", path, err)
					}
				}
				assertYAMLEqual(t, originalFiles[i], result)
			}
		}
	})
}
