package values

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// TestComplexDirectoryStructures tests extraction in unusual directory structures
func TestComplexDirectoryStructures(t *testing.T) {
	tests := []struct {
		name            string
		setupDirs       []string
		setupContents   [][]byte
		expectExtracted bool
		description     string
	}{
		{
			name: "symmetric tree structure",
			setupDirs: []string{
				"root/left/deep/service-a",
				"root/left/deep/service-b", 
				"root/right/deep/service-c",
				"root/right/deep/service-d",
			},
			setupContents: [][]byte{
				[]byte(`shared:
  company: acme
  monitoring: true
specific:
  name: service-a`),
				[]byte(`shared:
  company: acme
  monitoring: true
specific:
  name: service-b`),
				[]byte(`shared:
  company: acme
  monitoring: true
specific:
  name: service-c`),
				[]byte(`shared:
  company: acme
  monitoring: true
specific:
  name: service-d`),
			},
			expectExtracted: true,
			description:     "symmetric tree with shared config should extract at multiple levels",
		},
		{
			name: "asymmetric tree structure",
			setupDirs: []string{
				"app/frontend/components/header",
				"app/frontend/components/footer",
				"app/backend/api",
				"app/backend/worker",
				"app/database",
			},
			setupContents: [][]byte{
				[]byte(`frontend:
  framework: react
  version: 18
component:
  name: header
  type: ui`),
				[]byte(`frontend:
  framework: react
  version: 18
component:
  name: footer
  type: ui`),
				[]byte(`backend:
  language: go
  runtime: 1.21
service:
  name: api
  type: rest`),
				[]byte(`backend:
  language: go
  runtime: 1.21
service:
  name: worker
  type: async`),
				[]byte(`database:
  engine: postgresql
  version: 14
config:
  name: main-db`),
			},
			expectExtracted: true,
			description:     "asymmetric tree with component-level sharing",
		},
		{
			name: "deep single chain - no siblings",
			setupDirs: []string{
				"a/b/c/d/e/service",
			},
			setupContents: [][]byte{
				[]byte(`service:
  name: lonely-service
  config:
    isolated: true`),
			},
			expectExtracted: false,
			description:     "single service with no siblings should not extract anything",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, fullDirs := setupTempDirs(t, tc.setupDirs...)
			setupValuesFiles(t, fullDirs, tc.setupContents)
			
			rootDir := filepath.Dir(fullDirs[0])
			// Find the actual root by going up until we find the first common ancestor
			for strings.Contains(rootDir, "/") {
				if filepath.Base(rootDir) == "root" || filepath.Base(rootDir) == "app" || filepath.Base(rootDir) == "a" {
					break
				}
				rootDir = filepath.Dir(rootDir)
			}
			
			created, err := ExtractCommonRecursive(rootDir)
			if err != nil {
				t.Fatalf("ExtractCommonRecursive failed: %v", err)
			}
			
			hasExtracted := len(created) > 0
			if hasExtracted != tc.expectExtracted {
				t.Errorf("expected extraction=%v, got=%v. Created: %v", tc.expectExtracted, hasExtracted, created)
			}
			
			t.Logf("%s: extracted %d files", tc.description, len(created))
		})
	}
}

// TestNonSiblingDetection tests that non-sibling files are properly rejected
func TestNonSiblingDetection(t *testing.T) {
	tests := []struct {
		name        string
		setupDirs   []string
		expectError string
	}{
		{
			name: "different parents should fail",
			setupDirs: []string{
				"proj-a/services/auth",
				"proj-b/services/auth",
			},
			expectError: "same parent directory",
		},
		{
			name: "different depths under same root should fail",
			setupDirs: []string{
				"company/services/api",
				"company/deep/nested/services/worker",
			},
			expectError: "same parent directory", 
		},
		{
			name: "cousins should fail",
			setupDirs: []string{
				"org/team-a/backend/api",
				"org/team-b/backend/api",
			},
			expectError: "same parent directory",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, fullDirs := setupTempDirs(t, tc.setupDirs...)
			content := [][]byte{
				[]byte("shared: value\nunique: a"),
				[]byte("shared: value\nunique: b"),
			}
			paths := setupValuesFiles(t, fullDirs, content)
			
			_, err := ExtractCommonN(paths)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			
			if !strings.Contains(err.Error(), tc.expectError) {
				t.Fatalf("expected error containing %q, got %q", tc.expectError, err.Error())
			}
		})
	}
}

// TestMixedSharingAcrossLevels tests scenarios where sharing occurs at different levels
func TestMixedSharingAcrossLevels(t *testing.T) {
	t.Run("sharing at multiple hierarchy levels", func(t *testing.T) {
		// Create a complex hierarchy where sharing happens at different levels
		dirs := []string{
			"company/prod/region-us/service-a",
			"company/prod/region-us/service-b", 
			"company/prod/region-eu/service-c",
			"company/prod/region-eu/service-d",
			"company/staging/region-us/service-a",
			"company/staging/region-us/service-b",
		}
		
		contents := [][]byte{
			// Prod US services
			[]byte(`global:
  company: acme
  environment: prod
  region: us
monitoring:
  enabled: true
  datacenter: us-east-1
service:
  name: service-a
  replicas: 3`),
			[]byte(`global:
  company: acme
  environment: prod
  region: us
monitoring:
  enabled: true
  datacenter: us-east-1
service:
  name: service-b
  replicas: 2`),
			
			// Prod EU services
			[]byte(`global:
  company: acme
  environment: prod
  region: eu
monitoring:
  enabled: true
  datacenter: eu-west-1
service:
  name: service-c
  replicas: 2`),
			[]byte(`global:
  company: acme
  environment: prod
  region: eu
monitoring:
  enabled: true
  datacenter: eu-west-1
service:
  name: service-d
  replicas: 1`),
			
			// Staging US services  
			[]byte(`global:
  company: acme
  environment: staging
  region: us
monitoring:
  enabled: false
  datacenter: us-east-1
service:
  name: service-a
  replicas: 1`),
			[]byte(`global:
  company: acme
  environment: staging
  region: us
monitoring:
  enabled: false
  datacenter: us-east-1
service:
  name: service-b
  replicas: 1`),
		}
		
		_, fullDirs := setupTempDirs(t, dirs...)
		setupValuesFiles(t, fullDirs, contents)
		
		rootDir := filepath.Dir(fullDirs[0])
		for filepath.Base(rootDir) != "company" {
			rootDir = filepath.Dir(rootDir)
		}
		
		created, err := ExtractCommonRecursive(rootDir)
		if err != nil {
			t.Fatalf("ExtractCommonRecursive failed: %v", err)
		}
		
		t.Logf("Complex hierarchy created %d common files:", len(created))
		for _, path := range created {
			relPath, _ := filepath.Rel(rootDir, path)
			t.Logf("  - %s", relPath)
		}
		
		// Should create common files at multiple levels:
		// - region level (for services in same region)  
		// - environment level (for same environments)
		// - company level (for global company config)
		if len(created) < 3 {
			t.Errorf("expected at least 3 levels of common files, got %d", len(created))
		}
	})
}

// TestZeroAndSingleFileScenarios tests edge cases with very few files
func TestZeroAndSingleFileScenarios(t *testing.T) {
	tests := []struct {
		name        string
		fileCount   int
		expectError string
	}{
		{
			name:        "zero files",
			fileCount:   0,
			expectError: "need at least 2 files",
		},
		{
			name:        "single file", 
			fileCount:   1,
			expectError: "need at least 2 files",
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var paths []string
			if tc.fileCount > 0 {
				_, dirs := setupTempDirs(t, "single/service")
				paths = setupValuesFiles(t, dirs, [][]byte{[]byte("service: solo")})
			}
			
			_, err := ExtractCommonN(paths)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			
			if !strings.Contains(err.Error(), tc.expectError) {
				t.Fatalf("expected error containing %q, got %q", tc.expectError, err.Error())
			}
		})
	}
}

// TestIdenticalFilesExtraction tests extraction when files are identical
func TestIdenticalFilesExtraction(t *testing.T) {
	t.Run("identical files should extract everything to common", func(t *testing.T) {
		identicalContent := []byte(`app:
  name: shared-service
  version: 1.0.0
  config:
    replicas: 3
    resources:
      memory: 512Mi
      cpu: 250m
database:
  type: postgresql
  host: db.company.com
  port: 5432`)
		
		_, dirs := setupTempDirs(t, "services/replica-a", "services/replica-b", "services/replica-c")
		contents := [][]byte{identicalContent, identicalContent, identicalContent}
		paths := setupValuesFiles(t, dirs, contents)
		
		commonPath, err := ExtractCommonN(paths)
		if err != nil {
			t.Fatalf("ExtractCommonN failed: %v", err)
		}
		
		// Common should contain everything
		common := mustReadFile(t, commonPath)
		assertYAMLEqual(t, identicalContent, common)
		
		// Each file should be empty or nearly empty
		for i, path := range paths {
			updated := mustReadFile(t, path)
			if len(updated) > 10 { // Allow for minimal structure like {}
				t.Errorf("expected minimal content in file %d after extraction, got %d bytes", i, len(updated))
			}
		}
	})
}

// TestLargeNumberOfFiles tests extraction with many files
func TestLargeNumberOfFiles(t *testing.T) {
	t.Run("many files with common structure", func(t *testing.T) {
		const numFiles = 50
		
		var dirs []string
		var contents [][]byte
		
		for i := 0; i < numFiles; i++ {
			serviceName := fmt.Sprintf("svc-%02d", i)
			dirs = append(dirs, filepath.Join("services", serviceName))
			
			content := []byte(fmt.Sprintf(`global:
  company: mega-corp
  monitoring:
    enabled: true
    prometheus: true
    grafana: true
service:
  name: %s
  port: %d
  replicas: %d`, serviceName, 8080+i, 1+(i%5)))
			contents = append(contents, content)
		}
		
		_, fullDirs := setupTempDirs(t, dirs...)
		paths := setupValuesFiles(t, fullDirs, contents)
		
		commonPath, err := ExtractCommonN(paths)
		if err != nil {
			t.Fatalf("ExtractCommonN failed with %d files: %v", numFiles, err)
		}
		
		// Validate that common contains the shared global config
		common := mustReadFile(t, commonPath)
		if len(common) < 50 { // Should have substantial shared content
			t.Errorf("expected substantial common content, got %d bytes", len(common))
		}
		
		// Validate merge property for first few files (don't check all 50 for performance)
		for i := 0; i < min(5, len(contents)); i++ {
			updated := mustReadFile(t, paths[i])
			validateMergeProperty(t, contents[i], common, updated)
		}
		
		t.Logf("Successfully processed %d files, extracted %d bytes of common config", numFiles, len(common))
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
