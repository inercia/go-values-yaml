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

// ExtractCommonRecursive scans the directory tree rooted at root and progressively
// extracts common structures bottom-up.
//
// Algorithm:
// - Walk the tree to list all directories and their immediate child directories.
// - Repeat in passes from deepest parents to shallowest:
//   - For each parent directory, collect its direct child directories that currently
//     contain a values.yaml (including ones created in prior passes).
//   - If two or more child values.yaml files exist, run ExtractCommonN on them to
//     produce/overwrite the parent values.yaml and update children with remainders.
//   - Newly created parent values.yaml files make that parent eligible in the next pass
//     to be grouped with its own siblings at a higher level.
//
// - Stops when a full pass creates no new parent values.yaml files.
//
// Returns the sorted list of parent values.yaml paths that were created during the run.
func ExtractCommonRecursive(root string, opts ...Option) ([]string, error) {
	// Validate root
	st, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("root is not a directory: %s", root)
	}

	// Discover directories and parent->children relationships
	dirs := make(map[string]struct{})
	parentToChildren := make(map[string][]string)
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
			parentToChildren[parent] = append(parentToChildren[parent], path)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Track which directories currently have a values.yaml file
	hasValues := make(map[string]bool)
	for dir := range dirs {
		if fi, err := os.Stat(filepath.Join(dir, "values.yaml")); err == nil && !fi.IsDir() {
			hasValues[dir] = true
		}
	}

	// Prepare parents ordered by depth (deepest first)
	parents := make([]string, 0, len(parentToChildren))
	for p := range parentToChildren {
		parents = append(parents, p)
	}
	sort.Slice(parents, func(i, j int) bool {
		return pathDepth(parents[i]) > pathDepth(parents[j])
	})

	// Iteratively extract upwards
	createdSet := make(map[string]struct{})
	for {
		createdInPass := 0
		for _, parent := range parents {
			children := parentToChildren[parent]
			if len(children) == 0 {
				continue
			}
			paths := make([]string, 0, len(children))
			for _, child := range children {
				if hasValues[child] {
					vp := filepath.Join(child, "values.yaml")
					if fi, err := os.Stat(vp); err == nil && !fi.IsDir() {
						paths = append(paths, vp)
					}
				}
			}
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
			// Mark parent as now having a values file (if not already)
			if !hasValues[parent] {
				hasValues[parent] = true
				createdInPass++
			}
			createdSet[commonPath] = struct{}{}
		}
		if createdInPass == 0 {
			break
		}
	}

	// Collect and sort created paths
	created := make([]string, 0, len(createdSet))
	for p := range createdSet {
		created = append(created, p)
	}
	sort.Strings(created)
	return created, nil
}

// pathDepth returns the number of ancestors between p and the filesystem root.
func pathDepth(p string) int {
	depth := 0
	for {
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		depth++
		p = parent
	}
	return depth
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
