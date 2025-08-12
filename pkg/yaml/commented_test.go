package yaml

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	syaml "sigs.k8s.io/yaml"
)

func TestCommentedOut_Fixtures(t *testing.T) {
	cases := []struct {
		name      string
		regular   string
		commented string
	}{
		{
			name:      "regular_annotations_commented",
			regular:   filepath.Join("fixtures", "1-regular.yaml"),
			commented: filepath.Join("fixtures", "1-regular-commented.yaml"),
		},
		{
			name:      "nested_lists_and_labels_commented",
			regular:   filepath.Join("fixtures", "2-nested-lists.yaml"),
			commented: filepath.Join("fixtures", "2-nested-lists-commented.yaml"),
		},
		{
			name:      "deep_maps_selective_comments",
			regular:   filepath.Join("fixtures", "3-deep-maps.yaml"),
			commented: filepath.Join("fixtures", "3-deep-maps-commented.yaml"),
		},
		{
			name:      "partial_nested_map",
			regular:   filepath.Join("fixtures", "4-partial-nested.yaml"),
			commented: filepath.Join("fixtures", "4-partial-nested-commented.yaml"),
		},
		{
			name:      "list_removed_commented_whole_key",
			regular:   filepath.Join("fixtures", "5-lists.yaml"),
			commented: filepath.Join("fixtures", "5-lists-commented.yaml"),
		},
		{
			name:      "empty_maps_and_scalars",
			regular:   filepath.Join("fixtures", "6-empty-maps-scalars.yaml"),
			commented: filepath.Join("fixtures", "6-empty-maps-scalars-commented.yaml"),
		},
		{
			name:      "nonmap_root_entire_doc_commented",
			regular:   filepath.Join("fixtures", "7-nonmap-root.yaml"),
			commented: filepath.Join("fixtures", "7-nonmap-root-commented.yaml"),
		},
		{
			name:      "top_level_multiple_deletions",
			regular:   filepath.Join("fixtures", "8-top-level-multiple.yaml"),
			commented: filepath.Join("fixtures", "8-top-level-multiple-commented.yaml"),
		},
		{
			name:      "nested_deletions_mixed",
			regular:   filepath.Join("fixtures", "9-nested-deletions.yaml"),
			commented: filepath.Join("fixtures", "9-nested-deletions-commented.yaml"),
		},
		{
			name:      "nil_whole_branch",
			regular:   filepath.Join("fixtures", "10-nil-whole-branch.yaml"),
			commented: filepath.Join("fixtures", "10-nil-whole-branch-commented.yaml"),
		},
		{
			name:      "nested_list_comment_whole",
			regular:   filepath.Join("fixtures", "11-nested-list-commented.yaml"),
			commented: filepath.Join("fixtures", "11-nested-list-commented-commented.yaml"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			regularBytes, err := os.ReadFile(tc.regular)
			if err != nil {
				t.Fatalf("read regular: %v", err)
			}
			commentedBytes, err := os.ReadFile(tc.commented)
			if err != nil {
				t.Fatalf("read commented: %v", err)
			}

			var full any
			if err := syaml.Unmarshal(regularBytes, &full); err != nil {
				t.Fatalf("unmarshal regular: %v", err)
			}
			var masked any
			if err := syaml.Unmarshal(commentedBytes, &masked); err != nil {
				t.Fatalf("unmarshal commented: %v", err)
			}

			got, err := CommentedOut(full, masked)
			if err != nil {
				t.Fatalf("CommentedOut error: %v", err)
			}
			if string(got) != string(commentedBytes) {
				t.Fatalf("output mismatch for %s\n---- got ----\n%s\n---- expect ----\n%s", tc.name, string(got), string(commentedBytes))
			}
		})
	}
}

// Assert the produced YAML for non-commented parts is valid YAML
// NOTE: This test remains to ensure non-fixture behavior still valid.
func TestCommentedOut_ProducesValidYAMLWhenUncommentedOnly(t *testing.T) {
	full := map[string]any{"a": 1, "b": 2}
	masked := map[string]any{"a": 1}
	got, err := CommentedOut(full, masked)
	if err != nil {
		t.Fatal(err)
	}
	// Remove commented lines and ensure remaining parses and matches masked
	filtered := filterUncommented(got)
	var m any
	if err := syaml.Unmarshal(filtered, &m); err != nil {
		t.Fatalf("filtered YAML invalid: %v\n%s", err, string(filtered))
	}
	mb, _ := syaml.Marshal(masked)
	var m2 any
	_ = syaml.Unmarshal(mb, &m2)
	m = normalizeNilToEmpty(m)
	m2 = normalizeNilToEmpty(m2)
	if !deepEqual(m, m2) {
		t.Fatalf("remaining YAML doesn't match masked\nremain:\n%s\nmasked:\n%s", string(filtered), string(mb))
	}
}

func filterUncommented(in []byte) []byte {
	out := make([]byte, 0, len(in))
	for _, ln := range bytes.Split(in, []byte("\n")) {
		if len(ln) == 0 {
			out = append(out, '\n')
			continue
		}
		trim := bytes.TrimSpace(ln)
		if bytes.HasPrefix(trim, []byte("#")) {
			continue
		}
		out = append(out, ln...)
		out = append(out, '\n')
	}
	return out
}

func normalizeNilToEmpty(v any) any {
	switch t := v.(type) {
	case nil:
		return map[string]any{}
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, vv := range t {
			m[k] = normalizeNilToEmpty(vv)
		}
		return m
	case []any:
		l := make([]any, len(t))
		for i := range t {
			l[i] = normalizeNilToEmpty(t[i])
		}
		return l
	default:
		return v
	}
}
