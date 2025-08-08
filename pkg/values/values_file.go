package values

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

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
	if err != nil {
		return "", err
	}
	y2, err := os.ReadFile(path2)
	if err != nil {
		return "", err
	}

	// Compute common and remainders using pkg/yaml
	commonY, u1Y, u2Y, err := yamllib.ExtractCommon(y1, y2, yamllib.WithIncludeEqualListsInCommon(options.IncludeEqualListsInCommon))
	if err != nil {
		return "", err
	}

	// If common is empty ({}), do nothing
	if isEmptyYAML(commonY) {
		return "", ErrNoCommon
	}

	// Write common and updated files atomically
	commonPath = filepath.Join(p1, "values.yaml")
	if err := writeFileAtomic(commonPath, commonY, 0o644); err != nil {
		return "", err
	}
	if err := writeFileAtomic(path1, u1Y, 0o644); err != nil {
		return "", err
	}
	if err := writeFileAtomic(path2, u2Y, 0o644); err != nil {
		return "", err
	}

	return commonPath, nil
}

// ExtractCommonN performs the same operation as ExtractCommon but for N sibling
// values.yaml files. It writes the common structure to the shared parent directory
// as values.yaml and updates each provided file with its remainder.
// Returns the path to the common file or ErrNoCommon if there is no common content.
func ExtractCommonN(paths []string, opts ...Option) (commonPath string, err error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}
	if len(paths) < 2 {
		return "", fmt.Errorf("need at least 2 files, got %d", len(paths))
	}
	// Validate names and gather parent
	parents := make(map[string]struct{})
	for _, p := range paths {
		if filepath.Base(p) != "values.yaml" {
			return "", fmt.Errorf("file must be named values.yaml: %s", p)
		}
		if err := assertFileExists(p); err != nil {
			return "", err
		}
		parents[filepath.Dir(filepath.Dir(p))] = struct{}{}
	}
	if len(parents) != 1 {
		return "", fmt.Errorf("all files must share the same parent directory one level up")
	}
	var parent string
	for k := range parents {
		parent = k
	}

	// Read content
	yams := make([][]byte, len(paths))
	for i, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		yams[i] = b
	}

	// Compute common and remainders
	commonY, remainders, err := yamllib.ExtractCommonN(yams, yamllib.WithIncludeEqualListsInCommon(options.IncludeEqualListsInCommon))
	if err != nil {
		return "", err
	}
	if isEmptyYAML(commonY) {
		return "", ErrNoCommon
	}

	// Write outputs
	commonPath = filepath.Join(parent, "values.yaml")
	if err := writeFileAtomic(commonPath, commonY, 0o644); err != nil {
		return "", err
	}
	for i, p := range paths {
		if err := writeFileAtomic(p, remainders[i], 0o644); err != nil {
			return "", err
		}
	}
	return commonPath, nil
}

// ExtractCommonRecursive scans the directory tree rooted at root and, for every
// directory whose immediate children include two or more leaf directories that each
// contain a values.yaml file, extracts the common structure across those leaf
// values into a new values.yaml in the parent directory. Each child values.yaml
// is updated to its remainder.
//
// Behavior:
// - Only leaf directories (directories with no subdirectories) are considered as children.
// - Sibling groups with fewer than two values.yaml files are ignored.
// - Groups with no common content are skipped without error.
// - Returns a sorted list of the parent values.yaml paths created.
func ExtractCommonRecursive(root string, opts ...Option) ([]string, error) {
	// Ensure root exists and is a directory
	st, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("root is not a directory: %s", root)
	}

	// Collect all directories and mark which have child directories
	dirs := make(map[string]struct{})
	hasChildDir := make(map[string]bool)
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		dirs[path] = struct{}{}
		if path != root {
			parent := filepath.Dir(path)
			hasChildDir[parent] = true
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Identify leaf directories under root
	leafDirs := make(map[string]struct{})
	for d := range dirs {
		if !hasChildDir[d] {
			leafDirs[d] = struct{}{}
		}
	}

	// Group leaf directories that contain a values.yaml by their parent directory
	parentToValues := make(map[string][]string)
	for d := range leafDirs {
		vp := filepath.Join(d, "values.yaml")
		fi, err := os.Stat(vp)
		if err != nil || fi.IsDir() {
			continue
		}
		parent := filepath.Dir(d)
		parentToValues[parent] = append(parentToValues[parent], vp)
	}

	// For each parent with at least two values.yaml children, extract common
	created := make([]string, 0)
	for _, paths := range parentToValues {
		if len(paths) < 2 {
			continue
		}
		commonPath, err := ExtractCommonN(paths, opts...)
		if err != nil {
			if errors.Is(err, ErrNoCommon) {
				continue
			}
			return nil, err
		}
		created = append(created, commonPath)
	}

	// Sort for deterministic order
	sort.Strings(created)
	return created, nil
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
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(name)
	}()

	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
