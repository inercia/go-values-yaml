package values

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	yamllib "github.com/inercia/go-values-yaml/pkg/yaml"
	syaml "sigs.k8s.io/yaml"
)

// ErrNoCommon is returned when two values.yaml files have no common structure.
var ErrNoCommon = errors.New("no common values found")

// Options controls how common structures are extracted for values.yaml files.
// Currently it wraps the YAML-level options used by pkg/yaml.
type Options struct {
	// IncludeEqualListsInCommon controls whether lists that are equal across both
	// values files should be extracted into the common file. Default true.
	IncludeEqualListsInCommon bool
}

// Option is a functional option for file-based extraction.
type Option func(*Options)

// WithIncludeEqualListsInCommon forwards the option to the underlying YAML extractor.
func WithIncludeEqualListsInCommon(include bool) Option {
	return func(o *Options) { o.IncludeEqualListsInCommon = include }
}

func defaultOptions() Options {
	return Options{IncludeEqualListsInCommon: true}
}

// ExtractCommon reads two values.yaml files and extracts their common structure into
// a new values.yaml placed one directory above both files. The original files are
// rewritten to only contain their respective remainders (i.e., without the common part).
//
// Requirements and behavior:
// - Both input paths must be named "values.yaml" and exist.
// - Both must be at the same depth and share the same parent directory (i.e., siblings).
// - The common file is written at the shared parent directory as "values.yaml".
// - If no common structure exists, this function returns ErrNoCommon and leaves files unchanged.
// - The merge property holds: merge(updated, common) reconstructs each original.
func ExtractCommon(path1, path2 string, opts ...Option) (commonPath string, err error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	if filepath.Base(path1) != "values.yaml" || filepath.Base(path2) != "values.yaml" {
		return "", fmt.Errorf("both files must be named values.yaml: got %q and %q", filepath.Base(path1), filepath.Base(path2))
	}
	if err := assertFileExists(path1); err != nil {
		return "", err
	}
	if err := assertFileExists(path2); err != nil {
		return "", err
	}

	dir1 := filepath.Dir(path1)
	dir2 := filepath.Dir(path2)
	p1 := filepath.Dir(dir1)
	p2 := filepath.Dir(dir2)
	if p1 != p2 {
		return "", fmt.Errorf("both files must share the same parent directory: got %q vs %q", p1, p2)
	}

	// Read YAML files
	y1, err := os.ReadFile(path1)
	if err != nil { return "", err }
	y2, err := os.ReadFile(path2)
	if err != nil { return "", err }

	// Compute common and remainders using pkg/yaml
	commonY, u1Y, u2Y, err := yamllib.ExtractCommon(y1, y2, yamllib.WithIncludeEqualListsInCommon(options.IncludeEqualListsInCommon))
	if err != nil { return "", err }

	// If common is empty ({}), do nothing
	if isEmptyYAML(commonY) {
		return "", ErrNoCommon
	}

	// Write common and updated files atomically
	commonPath = filepath.Join(p1, "values.yaml")
	if err := writeFileAtomic(commonPath, commonY, 0o644); err != nil { return "", err }
	if err := writeFileAtomic(path1, u1Y, 0o644); err != nil { return "", err }
	if err := writeFileAtomic(path2, u2Y, 0o644); err != nil { return "", err }

	return commonPath, nil
}

func assertFileExists(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("path is a directory, expected file: %s", path)
	}
	return nil
}

func isEmptyYAML(b []byte) bool {
	var v any
	if err := syaml.Unmarshal(b, &v); err != nil {
		// On parse error treat as non-empty to avoid accidental no-ops
		return false
	}
	return isEmpty(v)
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	switch vv := v.(type) {
	case map[string]any:
		return len(vv) == 0
	case []any:
		return len(vv) == 0
	default:
		return false
	}
}

// writeFileAtomic writes data to a temp file in the same directory and renames it in place.
func writeFileAtomic(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".values-*.tmp")
	if err != nil { return err }
	name := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(name)
	}()

	if _, err := tmp.Write(data); err != nil { return err }
	if err := tmp.Chmod(perm); err != nil { return err }
	if err := tmp.Sync(); err != nil { return err }

	if err := tmp.Close(); err != nil { return err }
	return os.Rename(name, path)
}
