package values

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/inercia/go-values-yaml/pkg/yaml"
	"github.com/stretchr/testify/assert"
)

func TestValues_DeepCopyInto(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    Values
	}{
		{
			name: "test",
			v: Values{
				"httpProxy": map[string]interface{}{
					"annotations": map[string]interface{}{
						"test": "test",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			other := Values{}
			tt.v.DeepCopyInto(&other)
			assert.EqualValues(t, tt.v, other)
		})
	}
}

func TestValues_Merge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		initial   Values
		overwrite Values
		expected  Values
	}{
		{
			name: "simple",
			initial: Values{
				"httpProxy": Values{
					"annotations": Values{
						"test": "test",
					},
				},
				"config": "1234",
			},
			overwrite: Values{
				"httpProxy": Values{
					"annotations": Values{
						"test2": "test2",
					},
				},
				"config": "0987",
			},
			expected: Values{
				"httpProxy": Values{
					"annotations": Values{
						"test":  "test",
						"test2": "test2",
					},
				},
				"config": "0987",
			},
		},
		{
			name: "delete",
			initial: Values{
				"httpProxy": Values{
					"annotations": Values{
						"test": "test",
					},
				},
				"route53": Values{
					"some-config": "some-value",
				},
				"config": "1234",
			},
			overwrite: Values{
				"httpProxy": Values{
					"annotations": Values{
						"test2": "test2",
					},
				},
				"route53": nil, // remove the "route53" section
				"config":  "0987",
			},
			expected: Values{
				"httpProxy": Values{
					"annotations": Values{
						"test":  "test",
						"test2": "test2",
					},
				},
				"route53": nil,
				"config":  "0987",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := tt.initial.Merge(&tt.overwrite)
			assert.True(t, tt.expected.EqualYAML(*merged),
				yaml.DiffYAML(tt.expected.MustToYAML(), merged.MustToYAML()))
		})
	}
}

func TestValues_Lookup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		values  Values
		key     string
		want    interface{}
		wantErr error
	}{
		{
			name: "simple string",
			values: Values{
				"simple": "value",
			},
			key:  "simple",
			want: "value",
		},
		{
			name: "nested map",
			values: Values{
				"parent": Values{
					"child": "value",
				},
			},
			key:  "parent.child",
			want: "value",
		},
		{
			name: "array index",
			values: Values{
				"array": []interface{}{"first", "second", "third"},
			},
			key:  "array[1]",
			want: "second",
		},
		{
			name: "string array",
			values: Values{
				"strings": []string{"a", "b", "c"},
			},
			key:  "strings[1]",
			want: "b",
		},
		{
			name: "int array",
			values: Values{
				"numbers": []int{1, 2, 3},
			},
			key:  "numbers[1]",
			want: 2, // Changed from "2" to 2 - now returns raw int
		},
		{
			name: "nested array map",
			values: Values{
				"complex": []interface{}{
					Values{"key": "value1"},
					Values{"key": "value2"},
				},
			},
			key:  "complex[1].key",
			want: "value2",
		},
		{
			name: "key not found",
			values: Values{
				"exists": "value",
			},
			key:     "nonexistent",
			wantErr: ErrKeyNotFound,
		},
		{
			name: "array index out of bounds",
			values: Values{
				"array": []interface{}{"only"},
			},
			key:     "array[1]",
			wantErr: ErrIndexOutOfBounds,
		},
		{
			name: "malformed index",
			values: Values{
				"array": []interface{}{"value"},
			},
			key:     "array[invalid]",
			wantErr: ErrMalformedIndex,
		},
		{
			name: "deep nested structure",
			values: Values{
				"level1": Values{
					"level2": []interface{}{
						Values{
							"level3": "deep value",
						},
					},
				},
			},
			key:  "level1.level2[0].level3",
			want: "deep value",
		},
		{
			name: "mixed types",
			values: Values{
				"mixed": []interface{}{
					123,
					"string",
					Values{"key": "value"},
				},
			},
			key:  "mixed[0]",
			want: 123, // Changed from "123" to 123 - now returns raw int
		},
		{
			name: "map string interface",
			values: Values{
				"mapsi": map[string]interface{}{
					"key": "value",
				},
			},
			key:  "mapsi.key",
			want: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.values.Lookup(tt.key)

			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr) || strings.Contains(err.Error(), tt.wantErr.Error()),
					"expected error containing %v, got %v", tt.wantErr, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValues_Set(t *testing.T) {
	tests := []struct {
		name     string
		initial  Values
		key      string
		value    interface{}
		expected Values
		wantErr  bool
	}{
		{
			name:     "Set simple key",
			initial:  Values{},
			key:      "foo",
			value:    "bar",
			expected: Values{"foo": "bar"},
		},
		{
			name:     "Set nested key",
			initial:  Values{},
			key:      "foo.bar",
			value:    "baz",
			expected: Values{"foo": Values{"bar": "baz"}},
		},
		{
			name:     "Set array index",
			initial:  Values{},
			key:      "foo[0]",
			value:    "bar",
			expected: Values{"foo": []interface{}{"bar"}},
		},
		{
			name:     "Set nested array index",
			initial:  Values{},
			key:      "foo[0].bar",
			value:    "baz",
			expected: Values{"foo": []interface{}{Values{"bar": "baz"}}},
		},
		{
			name: "Overwrite existing value",
			initial: Values{
				"foo": "old",
			},
			key:      "foo",
			value:    "new",
			expected: Values{"foo": "new"},
		},
		{
			name:     "Set high array index",
			initial:  Values{},
			key:      "foo[2]",
			value:    "bar",
			expected: Values{"foo": []interface{}{nil, nil, "bar"}},
		},
		{
			name: "Set in existing array",
			initial: Values{
				"foo": []interface{}{"first"},
			},
			key:      "foo[1]",
			value:    "second",
			expected: Values{"foo": []interface{}{"first", "second"}},
		},
		{
			name:    "Invalid index syntax",
			initial: Values{},
			key:     "foo[invalid]",
			value:   "bar",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.initial.Set(tt.key, tt.value)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, tt.initial)

			// Additional validation for specific cases
			if strings.Contains(tt.key, "[") {
				// Verify array access works after setting
				val, err := tt.initial.Lookup(tt.key)
				assert.NoError(t, err)
				assert.Equal(t, tt.value, val)
			}
		})
	}
}

func TestValues_Rebase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    Values
		base string
		want Values
	}{
		{
			name: "simple",
			v: Values{
				"httpProxy": Values{
					"annotations": Values{
						"test": "test",
					},
				},
				"config": "1234",
			},
			base: "new",
			want: Values{
				"new": Values{
					"httpProxy": Values{
						"annotations": Values{
							"test": "test",
						},
					},
					"config": "1234",
				},
			},
		},
		{
			name: "deeper tree",
			v: Values{
				"httpProxy": Values{
					"annotations": Values{
						"test": "test",
					},
				},
				"config": "1234",
			},
			base: "some.new.key",
			want: Values{
				"some": Values{
					"new": Values{
						"key": Values{
							"httpProxy": Values{
								"annotations": Values{
									"test": "test",
								},
							},
							"config": "1234",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.Rebase(tt.base)
			assert.EqualValues(t, tt.want, *got)
		})
	}
}

// TestTypeConversions tests the helper functions for type conversions
func TestTypeConversions(t *testing.T) {
	t.Parallel()

	t.Run("toInt conversions", func(t *testing.T) {
		tests := []struct {
			name    string
			input   interface{}
			want    int
			wantErr bool
		}{
			{"int", 42, 42, false},
			{"int32", int32(42), 42, false},
			{"int64", int64(42), 42, false},
			{"float32", float32(42.0), 42, false},
			{"float64", float64(42.0), 42, false},
			{"string valid", "42", 42, false},
			{"string invalid", "not a number", 0, true},
			{"unsupported type", []int{1, 2}, 0, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := toInt(tt.input)
				if tt.wantErr {
					assert.Error(t, err)
					assert.ErrorIs(t, err, ErrInvalidType)
					// For string conversion errors, the error will wrap strconv.Atoi error
					if _, ok := tt.input.(string); ok && tt.name == "string invalid" {
						assert.Contains(t, err.Error(), "invalid")
					}
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.want, got)
				}
			})
		}
	})

	t.Run("toString conversions", func(t *testing.T) {
		tests := []struct {
			name    string
			input   interface{}
			want    string
			wantErr bool
		}{
			{"string", "hello", "hello", false},
			{"int", 42, "42", false},
			{"int32", int32(42), "42", false},
			{"int64", int64(42), "42", false},
			{"float32", float32(42.5), "42.5", false},
			{"float64", float64(42.5), "42.5", false},
			{"bool true", true, "true", false},
			{"bool false", false, "false", false},
			{"unsupported type", []string{"a", "b"}, "", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := toString(tt.input)
				if tt.wantErr {
					assert.Error(t, err)
					assert.ErrorIs(t, err, ErrInvalidType)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.want, got)
				}
			})
		}
	})

	t.Run("toValues conversions", func(t *testing.T) {
		tests := []struct {
			name    string
			input   interface{}
			want    Values
			wantErr bool
		}{
			{
				name:  "Values type",
				input: Values{"key": "value"},
				want:  Values{"key": "value"},
			},
			{
				name:  "map[string]interface{}",
				input: map[string]interface{}{"key": "value"},
				want:  Values{"key": "value"},
			},
			{
				name:    "unsupported type",
				input:   "not a map",
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := toValues(tt.input)
				if tt.wantErr {
					assert.Error(t, err)
					assert.ErrorIs(t, err, ErrInvalidType)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.want, got)
				}
			})
		}
	})
}

// TestValues_DeepCopy tests the DeepCopy functionality
func TestValues_DeepCopy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    *Values
	}{
		{
			name: "nil values",
			v:    nil,
		},
		{
			name: "empty values",
			v:    &Values{},
		},
		{
			name: "simple values",
			v:    &Values{"key": "value"},
		},
		{
			name: "nested values",
			v: &Values{
				"parent": Values{
					"child": "value",
					"array": []interface{}{1, 2, 3},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copy := tt.v.DeepCopy()
			if tt.v == nil {
				assert.Nil(t, copy)
			} else {
				assert.EqualValues(t, tt.v, copy)
				// Ensure it's a deep copy
				if copy != nil && len(*copy) > 0 {
					assert.NotSame(t, tt.v, copy)
				}
			}
		})
	}
}

// TestValues_JSON tests JSON conversion methods
func TestValues_JSON(t *testing.T) {
	t.Parallel()

	v := Values{
		"string": "value",
		"number": 42,
		"nested": Values{
			"key": "value",
		},
	}

	t.Run("ToJSON", func(t *testing.T) {
		data, err := v.ToJSON()
		assert.NoError(t, err)
		assert.NotEmpty(t, data)

		// Verify it's valid JSON
		var result map[string]interface{}
		err = json.Unmarshal(data, &result)
		assert.NoError(t, err)
		assert.Equal(t, "value", result["string"])
	})

	t.Run("ToJSONIndented", func(t *testing.T) {
		data, err := v.ToJSONIndented()
		assert.NoError(t, err)
		assert.NotEmpty(t, data)
		assert.Contains(t, string(data), "\n")
		assert.Contains(t, string(data), "  ")
	})

	t.Run("NewValuesFromJSON", func(t *testing.T) {
		jsonData := `{"key": "value", "number": 42}`
		vals, err := NewValuesFromJSON([]byte(jsonData))
		assert.NoError(t, err)
		assert.Equal(t, "value", (*vals)["key"])
		assert.Equal(t, float64(42), (*vals)["number"]) // JSON numbers are float64
	})
}

// TestValues_LookupFirst tests the LookupFirst family of methods
func TestValues_LookupFirst(t *testing.T) {
	t.Parallel()

	v := Values{
		"primary":   "primary_value",
		"secondary": 42,
		"tertiary":  Values{"nested": "value"},
	}

	t.Run("LookupFirst", func(t *testing.T) {
		tests := []struct {
			name      string
			keys      []string
			wantValue interface{}
			wantKey   string
			wantErr   bool
		}{
			{
				name:      "finds first key",
				keys:      []string{"primary", "secondary"},
				wantValue: "primary_value",
				wantKey:   "primary",
			},
			{
				name:      "finds second key",
				keys:      []string{"missing", "secondary"},
				wantValue: 42,
				wantKey:   "secondary",
			},
			{
				name:    "all keys missing",
				keys:    []string{"missing1", "missing2"},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				val, key, err := v.LookupFirst(tt.keys)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.wantValue, val)
					assert.Equal(t, tt.wantKey, key)
				}
			})
		}
	})

	t.Run("LookupFirstString", func(t *testing.T) {
		val, key, err := v.LookupFirstString([]string{"missing", "primary"})
		assert.NoError(t, err)
		assert.Equal(t, "primary_value", val)
		assert.Equal(t, "primary", key)

		val, key, err = v.LookupFirstString([]string{"missing", "secondary"})
		assert.NoError(t, err)
		assert.Equal(t, "42", val)
		assert.Equal(t, "secondary", key)
	})

	t.Run("LookupFirstInt", func(t *testing.T) {
		val, key, err := v.LookupFirstInt([]string{"missing", "secondary"})
		assert.NoError(t, err)
		assert.Equal(t, 42, val)
		assert.Equal(t, "secondary", key)

		// Test string to int conversion
		v["stringnum"] = "123"
		val, key, err = v.LookupFirstInt([]string{"stringnum"})
		assert.NoError(t, err)
		assert.Equal(t, 123, val)
		assert.Equal(t, "stringnum", key)
	})
}

// TestValues_LookupValues tests the LookupValues method
func TestValues_LookupValues(t *testing.T) {
	t.Parallel()

	v := Values{
		"nested": Values{"key": "value"},
		"mapsi":  map[string]interface{}{"key": "value"},
		"string": "not a map",
	}

	t.Run("Values type", func(t *testing.T) {
		result, err := v.LookupValues("nested")
		assert.NoError(t, err)
		assert.Equal(t, Values{"key": "value"}, result)
	})

	t.Run("map[string]interface{} type", func(t *testing.T) {
		result, err := v.LookupValues("mapsi")
		assert.NoError(t, err)
		assert.Equal(t, Values{"key": "value"}, result)
	})

	t.Run("invalid type", func(t *testing.T) {
		_, err := v.LookupValues("string")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidType)
	})
}

// TestValues_Empty tests the Empty method
func TestValues_Empty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    Values
		want bool
	}{
		{"empty", Values{}, true},
		{"nil map", nil, true},
		{"not empty", Values{"key": "value"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.v.Empty())
		})
	}
}

// TestValues_SetEdgeCases tests edge cases in the Set method
func TestValues_SetEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		initial  Values
		key      string
		value    interface{}
		expected Values
		wantErr  bool
	}{
		{
			name:    "empty key",
			initial: Values{},
			key:     "",
			value:   "value",
			wantErr: true,
		},
		{
			name: "overwrite non-array with array",
			initial: Values{
				"foo": "string",
			},
			key:      "foo[0]",
			value:    "array_value",
			expected: Values{"foo": []interface{}{"array_value"}},
		},
		{
			name:    "deep nested creation",
			initial: Values{},
			key:     "a.b.c.d.e",
			value:   "deep",
			expected: Values{
				"a": Values{
					"b": Values{
						"c": Values{
							"d": Values{
								"e": "deep",
							},
						},
					},
				},
			},
		},
		{
			name:    "mixed array and map nesting",
			initial: Values{},
			key:     "list[0].map.list[1].value",
			value:   "complex",
			expected: Values{
				"list": []interface{}{
					Values{
						"map": Values{
							"list": []interface{}{
								nil,
								Values{"value": "complex"},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.initial.Set(tt.key, tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, tt.initial)
			}
		})
	}
}

// TestParseIndex tests the parseIndex helper function
func TestParseIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantKey   string
		wantIndex int
		wantErr   bool
	}{
		{"no index", "key", "key", -1, false},
		{"with index", "key[0]", "key", 0, false},
		{"index 10", "key[10]", "key", 10, false},
		{"missing close bracket", "key[0", "", -1, true},
		{"missing open bracket", "key0]", "", -1, true},
		{"invalid index", "key[abc]", "", -1, true},
		{"negative index", "key[-1]", "", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, index, err := parseIndex(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantKey, key)
				assert.Equal(t, tt.wantIndex, index)
			}
		})
	}
}
