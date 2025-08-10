package values

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractCommon_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		y1, y2   []byte
		wantErr  bool
		wantCommon []byte
	}{
		{
			name:    "empty files should return ErrNoCommon",
			y1:      []byte(""),
			y2:      []byte(""),
			wantErr: true,
		},
		{
			name:    "one empty one non-empty should return ErrNoCommon",
			y1:      []byte(""),
			y2:      []byte("key: value"),
			wantErr: true,
		},

		{
			name:    "only comments should return ErrNoCommon",
			y1:      []byte("# This is a comment\n# Another comment"),
			y2:      []byte("# Different comment"),
			wantErr: true,
		},
		{
			name: "unicode characters in values",
			y1: []byte(`app:
  name: "测试应用"
  emoji: "🚀"
  unicode: "ñáéíóú"`),
			y2: []byte(`app:
  name: "测试应用"
  emoji: "🚀"
  unicode: "ñáéíóú"
other: different`),
			wantErr: false,
			wantCommon: []byte(`app:
  emoji: 🚀
  name: 测试应用
  unicode: ñáéíóú`),
		},
		{
			name: "null values and different types",
			y1: []byte(`config:
  nullable: null
  number: 42
  boolean: true
  string: "text"`),
			y2: []byte(`config:
  nullable: null
  number: 42
  boolean: true
  string: "text"
extra: value`),
			wantErr: false,
			wantCommon: []byte(`config:
  boolean: true
  number: 42
  string: text`),
		},
		{
			name: "very deep nesting",
			y1: []byte(`level1:
  level2:
    level3:
      level4:
        level5:
          level6:
            common: shared
            unique1: value1`),
			y2: []byte(`level1:
  level2:
    level3:
      level4:
        level5:
          level6:
            common: shared
            unique2: value2`),
			wantErr: false,
			wantCommon: []byte(`level1:
  level2:
    level3:
      level4:
        level5:
          level6:
            common: shared`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, dirs := setupTempDirs(t, "a/b/x", "a/b/y")
			paths := setupValuesFiles(t, dirs, [][]byte{tc.y1, tc.y2})
			
			_, err := ExtractCommon(paths[0], paths[1])
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if err != ErrNoCommon {
					t.Fatalf("expected ErrNoCommon, got %v", err)
				}
				return
			}
			
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			if tc.wantCommon != nil {
				commonPath := filepath.Join(filepath.Dir(dirs[0]), "values.yaml")
				assertYAMLEqual(t, tc.wantCommon, mustReadFile(t, commonPath))
			}
		})
	}
}

func TestExtractCommonN_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		inputs   [][]byte
		wantErr  bool
		errMsg   string
	}{
		{
			name:    "zero files should error",
			inputs:  [][]byte{},
			wantErr: true,
			errMsg:  "need at least 2 files",
		},
		{
			name:    "single file should error",
			inputs:  [][]byte{[]byte("key: value")},
			wantErr: true,
			errMsg:  "need at least 2 files",
		},
		{
			name: "many files with common subset",
			inputs: [][]byte{
				[]byte("shared: common\nunique1: val1"),
				[]byte("shared: common\nunique2: val2"),
				[]byte("shared: common\nunique3: val3"),
				[]byte("shared: common\nunique4: val4"),
				[]byte("shared: common\nunique5: val5"),
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.inputs) == 0 {
				_, err := ExtractCommonN([]string{})
				if !tc.wantErr {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("expected error containing %q, got %q", tc.errMsg, err.Error())
				}
				return
			}

			dirs := make([]string, len(tc.inputs))
			for i := range tc.inputs {
				dirs[i] = filepath.Join("apps", string('a'+rune(i)))
			}
			_, fullDirs := setupTempDirs(t, dirs...)
			paths := setupValuesFiles(t, fullDirs, tc.inputs)

			_, err := ExtractCommonN(paths)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("expected error containing %q, got %q", tc.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestExtractCommon_InvalidYAML(t *testing.T) {
	tests := []struct {
		name    string
		y1, y2  []byte
		wantErr bool
	}{
		{
			name:    "malformed YAML in first file",
			y1:      []byte("invalid: [\n  broken"),
			y2:      []byte("valid: true"),
			wantErr: true,
		},
		{
			name:    "malformed YAML in second file", 
			y1:      []byte("valid: true"),
			y2:      []byte("invalid: {\n  broken"),
			wantErr: true,
		},
		{
			name:    "both files malformed",
			y1:      []byte("invalid: ["),
			y2:      []byte("also: {"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, dirs := setupTempDirs(t, "a/b/x", "a/b/y")
			paths := setupValuesFiles(t, dirs, [][]byte{tc.y1, tc.y2})
			
			_, err := ExtractCommon(paths[0], paths[1])
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for malformed YAML, got nil")
			}
		})
	}
}

func TestExtractCommon_FilePermissions(t *testing.T) {
	tests := []struct {
		name           string
		makeReadOnly   bool
		wantErr        bool
	}{
		{
			name:         "writable parent directory should succeed",
			makeReadOnly: false,
			wantErr:      false,
		},
		{
			name:         "read-only parent directory should fail",
			makeReadOnly: true,
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, dirs := setupTempDirs(t, "parent/x", "parent/y")
			y1 := []byte("shared: value\nunique1: val1")
			y2 := []byte("shared: value\nunique2: val2")
			paths := setupValuesFiles(t, dirs, [][]byte{y1, y2})
			
			parent := filepath.Dir(dirs[0])
			if tc.makeReadOnly {
				cleanup := makeReadOnly(t, parent)
				defer cleanup()
			}

			_, err := ExtractCommon(paths[0], paths[1])
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for read-only parent, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestExtractCommon_FilenameValidation(t *testing.T) {
	tests := []struct {
		name      string
		filename1 string
		filename2 string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "non-values.yaml filename should error",
			filename1: "config.yaml",
			filename2: "values.yaml",
			wantErr:   true,
			errMsg:    "must be named values.yaml",
		},
		{
			name:      "both non-values.yaml should error",
			filename1: "app.yaml",
			filename2: "config.yaml", 
			wantErr:   true,
			errMsg:    "must be named values.yaml",
		},
		{
			name:      "correct filenames should succeed",
			filename1: "values.yaml",
			filename2: "values.yaml",
			wantErr:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, dirs := setupTempDirs(t, "a/b/x", "a/b/y")
			
			p1 := filepath.Join(dirs[0], tc.filename1)
			p2 := filepath.Join(dirs[1], tc.filename2)
			mustWriteFile(t, p1, []byte("shared: value\nunique1: val1"))
			mustWriteFile(t, p2, []byte("shared: value\nunique2: val2"))

			_, err := ExtractCommon(p1, p2)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("expected error containing %q, got %q", tc.errMsg, err.Error())
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestExtractCommonRecursive_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr bool
	}{
		{
			name: "root is not a directory should error",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				filePath := filepath.Join(dir, "not-a-dir")
				mustWriteFile(t, filePath, []byte("content"))
				return filePath
			},
			wantErr: true,
		},
		{
			name: "empty directory should succeed with no output",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: false,
		},
		{
			name: "directory with no values.yaml files should succeed with no output",
			setup: func(t *testing.T) string {
				dir, dirs := setupTempDirs(t, "a/b", "a/c")
				mustWriteFile(t, filepath.Join(dirs[0], "other.yaml"), []byte("content: value"))
				mustWriteFile(t, filepath.Join(dirs[1], "config.yaml"), []byte("setting: true"))
				return dir
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.setup(t)
			
			created, err := ExtractCommonRecursive(root)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if tc.name == "empty directory should succeed with no output" || 
				   tc.name == "directory with no values.yaml files should succeed with no output" {
					if len(created) != 0 {
						t.Fatalf("expected no created files, got %v", created)
					}
				}
			}
		})
	}
}
