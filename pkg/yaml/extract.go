package yaml

import (
	"errors"
	"reflect"

	syaml "sigs.k8s.io/yaml"
)

// Options controls how common structures are extracted.
//
// IncludeEqualListsInCommon controls whether lists (YAML sequences) that are
// exactly equal in both inputs are considered part of the common structure.
// If false, even equal lists will remain in the updated outputs instead of in
// the common output. Default is true.
//
// Additional options can be added via the Option pattern.
type Options struct {
	IncludeEqualListsInCommon bool
}

// Option is a functional option for ExtractCommon.
type Option func(*Options)

// WithIncludeEqualListsInCommon sets whether equal lists should be considered common.
func WithIncludeEqualListsInCommon(include bool) Option {
	return func(o *Options) { o.IncludeEqualListsInCommon = include }
}

func defaultOptions() Options {
	return Options{IncludeEqualListsInCommon: true}
}

// ExtractCommon computes the common structure between two YAML documents and
// returns three YAML documents:
//  1. the common structure
//  2. the first document with the common structure removed (updated1)
//  3. the second document with the common structure removed (updated2)
//
// The operation satisfies the property that a deterministic deep-merge of
// (updated, common) reconstructs the original input.
func ExtractCommon(yaml1, yaml2 []byte, opts ...Option) ([]byte, []byte, []byte, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	var v1 any
	var v2 any
	if len(yaml1) > 0 {
		if err := syaml.Unmarshal(yaml1, &v1); err != nil {
			return nil, nil, nil, err
		}
	}
	if len(yaml2) > 0 {
		if err := syaml.Unmarshal(yaml2, &v2); err != nil {
			return nil, nil, nil, err
		}
	}

	common, r1, r2 := extractCommonValue(v1, v2, options)

	// Normalize: represent empty documents as {} rather than null
	common = normalizeDocRoot(common)
	r1 = normalizeDocRoot(r1)
	r2 = normalizeDocRoot(r2)

	// Marshal results to YAML
	commonY, err := syaml.Marshal(common)
	if err != nil {
		return nil, nil, nil, err
	}
	r1Y, err := syaml.Marshal(r1)
	if err != nil {
		return nil, nil, nil, err
	}
	r2Y, err := syaml.Marshal(r2)
	if err != nil {
		return nil, nil, nil, err
	}
	return commonY, r1Y, r2Y, nil
}

// extractCommonValue returns the common part between a and b, and the remainders
// of a and b after removing the common part. The merge property holds for the
// triplet (common, ra, rb): merge(ra, common) == a and merge(rb, common) == b.
func extractCommonValue(a, b any, options Options) (common any, ra any, rb any) {
	// Fast path: identical scalars or identical lists with option enabled.
	if isScalar(a) && isScalar(b) {
		if reflect.DeepEqual(a, b) {
			return a, nil, nil
		}
		return nil, a, b
	}

	aMap, aIsMap := asStringMap(a)
	bMap, bIsMap := asStringMap(b)
	if aIsMap && bIsMap {
		cMap := make(map[string]any)
		raMap := make(map[string]any)
		rbMap := make(map[string]any)

		// Collect union of keys
		seen := make(map[string]struct{})
		for k := range aMap {
			seen[k] = struct{}{}
		}
		for k := range bMap {
			seen[k] = struct{}{}
		}

		for k := range seen {
			av, aok := aMap[k]
			bv, bok := bMap[k]
			switch {
			case aok && bok:
				cc, rra, rrb := extractCommonValue(av, bv, options)
				if !isEmpty(cc) {
					cMap[k] = cc
				}
				if !isEmpty(rra) {
					raMap[k] = rra
				}
				if !isEmpty(rrb) {
					rbMap[k] = rrb
				}
			case aok && !bok:
				raMap[k] = av
			case !aok && bok:
				rbMap[k] = bv
			}
		}

		return mapOrNil(cMap), mapOrNil(raMap), mapOrNil(rbMap)
	}

	aList, aIsList := asList(a)
	bList, bIsList := asList(b)
	if aIsList && bIsList {
		if options.IncludeEqualListsInCommon && reflect.DeepEqual(aList, bList) {
			return aList, nil, nil
		}
		// No partial extraction from lists; treat as entirely different
		return nil, aList, bList
	}

	// Types differ or unsupported; treat as different
	if isZero(a) && !isZero(b) {
		return nil, nil, b
	}
	if !isZero(a) && isZero(b) {
		return nil, a, nil
	}
	if reflect.DeepEqual(a, b) {
		return a, nil, nil
	}
	return nil, a, b
}

func isZero(v any) bool {
	return v == nil
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	if m, ok := asStringMap(v); ok {
		return len(m) == 0
	}
	if l, ok := asList(v); ok {
		return len(l) == 0
	}
	return false
}

func asStringMap(v any) (map[string]any, bool) {
	if v == nil {
		return nil, false
	}
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	return nil, false
}

func asList(v any) ([]any, bool) {
	if v == nil {
		return nil, false
	}
	if l, ok := v.([]any); ok {
		return l, true
	}
	return nil, false
}

func isScalar(v any) bool {
	if v == nil {
		return false
	}
	switch v.(type) {
	case string, bool, int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8, float32, float64:
		return true
	default:
		return false
	}
}

func mapOrNil(m map[string]any) any {
	if len(m) == 0 {
		return nil
	}
	return m
}

func normalizeDocRoot(v any) any {
	if v == nil {
		return map[string]any{}
	}
	return v
}

// MergeYAML merges two YAML documents in-memory by deep-merging their structures
// with a "first wins on conflict" policy. This is primarily intended for tests
// to validate that merge(updated, common) equals original.
func MergeYAML(baseYAML, overlayYAML []byte) ([]byte, error) {
	var base any
	var overlay any
	if err := syaml.Unmarshal(baseYAML, &base); err != nil {
		return nil, err
	}
	if err := syaml.Unmarshal(overlayYAML, &overlay); err != nil {
		return nil, err
	}

	merged, err := mergeValues(base, overlay)
	if err != nil {
		return nil, err
	}
	return syaml.Marshal(merged)
}

func mergeValues(a, b any) (any, error) {
	if a == nil {
		return b, nil
	}
	if b == nil {
		return a, nil
	}
	if am, ok := a.(map[string]any); ok {
		if bm, ok := b.(map[string]any); ok {
			out := make(map[string]any, len(am)+len(bm))
			for k, v := range am {
				out[k] = v
			}
			for k, bv := range bm {
				if av, exists := out[k]; exists {
					mv, err := mergeValues(av, bv)
					if err != nil {
						return nil, err
					}
					out[k] = mv
				} else {
					out[k] = bv
				}
			}
			return out, nil
		}
		return nil, errors.New("type conflict: map vs non-map")
	}
	// For lists and scalars, prefer the first (base) value to preserve updated
	// semantics and avoid unintended replacements.
	return a, nil
}
