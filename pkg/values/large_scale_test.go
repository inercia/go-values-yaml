package values

import (
	"fmt"
	"path/filepath"
	"testing"
)

// TestLargeScaleExtraction tests extraction with many files in complex hierarchies
func TestLargeScaleExtraction(t *testing.T) {
	t.Run("large microservices deployment", func(t *testing.T) {
		// Setup: 12 microservices across 3 environments
		environments := []string{"dev", "staging", "prod"}
		services := []string{"auth", "users", "orders", "notifications"}

		var dirs []string
		var contents [][]byte

		for _, env := range environments {
			for _, svc := range services {
				dirs = append(dirs, fmt.Sprintf("company/%s/services/%s", env, svc))

				// Create realistic service configuration
				content := fmt.Sprintf(`global:
  company: acme-corp
  environment: %s
  monitoring:
    enabled: true
    prometheus: true
service:
  name: %s
  type: microservice
  port: %d
  replicas: %d
database:
  type: postgresql
  host: %s-db.company.com
  port: 5432`, env, svc, 8080+len(svc), getReplicasForEnv(env), env)
				contents = append(contents, []byte(content))
			}
		}

		_, fullDirs := setupTempDirs(t, dirs...)
		setupValuesFiles(t, fullDirs, contents)
		rootDir := filepath.Dir(fullDirs[0])

		created, err := ExtractCommonRecursive(rootDir)
		if err != nil {
			t.Fatalf("ExtractCommonRecursive failed: %v", err)
		}

		t.Logf("Created %d common files from %d original files", len(created), len(contents))
		for _, path := range created {
			t.Logf("  Created: %s", path)
		}

		// Verify we created some meaningful common files
		if len(created) < 1 {
			t.Errorf("expected at least 1 common file for large deployment, got %d", len(created))
		}
	})
}

// TestPartialSharingPatterns tests various patterns of partial sharing
func TestPartialSharingPatterns(t *testing.T) {
	tests := []struct {
		name         string
		setupContent func() [][]byte
		expectShared bool
		sharedAspect string
	}{
		{
			name: "shared infrastructure, different applications",
			setupContent: func() [][]byte {
				return [][]byte{
					[]byte(`infrastructure:
  kubernetes:
    version: 1.28
    cluster: prod-cluster
  monitoring:
    prometheus: true
app:
  name: web-frontend
  language: typescript`),
					[]byte(`infrastructure:
  kubernetes:
    version: 1.28
    cluster: prod-cluster
  monitoring:
    prometheus: true
app:
  name: api-backend
  language: go`),
					[]byte(`infrastructure:
  kubernetes:
    version: 1.28
    cluster: prod-cluster
  monitoring:
    prometheus: true
app:
  name: worker-service
  language: python`),
				}
			},
			expectShared: true,
			sharedAspect: "infrastructure configuration",
		},
		{
			name: "no sharing - completely different structures",
			setupContent: func() [][]byte {
				return [][]byte{
					[]byte(`database:
  postgresql:
    host: db1.company.com
app:
  api:
    endpoints: [users, auth]`),
					[]byte(`cache:
  redis:
    cluster: true
worker:
  queues: [high, normal, low]`),
					[]byte(`storage:
  s3:
    bucket: company-data
etl:
  schedule: daily`),
				}
			},
			expectShared: false,
			sharedAspect: "nothing",
		},
		{
			name: "deep nested sharing with surface differences",
			setupContent: func() [][]byte {
				return [][]byte{
					[]byte(`config:
  app:
    name: service-alpha
  database:
    connection:
      pool:
        min: 5
        max: 20
      timeout: 30s
  logging:
    level: info
    format: json`),
					[]byte(`config:
  app:
    name: service-beta
  database:
    connection:
      pool:
        min: 5
        max: 20
      timeout: 30s
  logging:
    level: debug
    format: json`),
				}
			},
			expectShared: true,
			sharedAspect: "database and logging structure",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content := tc.setupContent()

			dirs := make([]string, len(content))
			for i := range content {
				dirs[i] = fmt.Sprintf("services/svc-%c", 'a'+rune(i))
			}
			_, fullDirs := setupTempDirs(t, dirs...)
			paths := setupValuesFiles(t, fullDirs, content)

			commonPath, err := ExtractCommonN(paths)

			if !tc.expectShared {
				if err != ErrNoCommon {
					t.Fatalf("expected ErrNoCommon for %s, got %v", tc.sharedAspect, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ExtractCommonN failed for %s: %v", tc.sharedAspect, err)
			}

			// Validate that something meaningful was extracted
			common := mustReadFile(t, commonPath)
			if len(common) < 10 { // Sanity check
				t.Errorf("expected substantial common content for %s, got %d bytes", tc.sharedAspect, len(common))
			}

			// Validate merge property
			for i, originalContent := range content {
				updated := mustReadFile(t, paths[i])
				validateMergeProperty(t, originalContent, common, updated)
			}
		})
	}
}

// Helper function to get replica count based on environment
func getReplicasForEnv(env string) int {
	switch env {
	case "dev":
		return 1
	case "staging":
		return 2
	case "prod":
		return 5
	default:
		return 1
	}
}
