package values

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"dario.cat/mergo"
	syaml "sigs.k8s.io/yaml"

	"github.com/inercia/go-values-yaml/pkg/yaml"
)

const (
	SplitToken     = "."
	IndexCloseChar = "]"
	IndexOpenChar  = "["
)

var (
	ErrMalformedIndex    = errors.New("malformed index key")
	ErrInvalidIndexUsage = errors.New("invalid index key usage")
	ErrKeyNotFound       = errors.New("unable to find the key")
	ErrIndexOutOfBounds  = errors.New("index out of bounds")
	ErrInvalidType       = errors.New("invalid type conversion")
)

///////////////////////////////////////////////////////////////////////////////
// Type conversion helpers
///////////////////////////////////////////////////////////////////////////////

// toInt converts various numeric types to int
func toInt(v interface{}) (int, error) {
	switch val := v.(type) {
	case int:
		return val, nil
	case int32:
		return int(val), nil
	case int64:
		return int(val), nil
	case float32:
		return int(val), nil
	case float64:
		return int(val), nil
	case string:
		i, err := strconv.Atoi(val)
		if err != nil {
			return 0, fmt.Errorf("%w: %v", ErrInvalidType, err)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("%w: cannot convert %T to int", ErrInvalidType, v)
	}
}

// toString converts various types to string
func toString(v interface{}) (string, error) {
	switch val := v.(type) {
	case string:
		return val, nil
	case int:
		return strconv.Itoa(val), nil
	case int32:
		return strconv.Itoa(int(val)), nil
	case int64:
		return strconv.FormatInt(val, 10), nil
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(val), nil
	default:
		return "", fmt.Errorf("%w: cannot convert %T to string", ErrInvalidType, v)
	}
}

// toValues converts map[string]interface{} to Values
func toValues(v interface{}) (Values, error) {
	switch val := v.(type) {
	case Values:
		return val, nil
	case map[string]interface{}:
		return Values(val), nil
	default:
		return nil, fmt.Errorf("%w: cannot convert %T to Values", ErrInvalidType, v)
	}
}

///////////////////////////////////////////////////////////////////////////////

type ValuesAnnotation struct {
	Value string `json:"value,omitempty"`
}

// ValuesPath is a path to a value in the Values map.
// It is a string of keys separated by the SplitToken.
// For example, "httpProxy.annotations.trans.id" is a valid ValuesPath.
type ValuesPath string

///////////////////////////////////////////////////////////////////////////////

// Values is a map of values that can be used to render the CGW Helm chart.
// These values are specific to the CGW Helm chart.
type Values map[string]interface{}

// NewValues creates a new Values instance.
func NewValues() *Values {
	return &Values{}
}

// NewValuesFromMap creates a new Values instance from a YAML document.
func NewValuesFromYAML(b []byte) (*Values, error) {
	v := Values{}
	if err := syaml.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// NewValuesFromJSON creates a new Values instance from a JSON document.
func NewValuesFromJSON(b []byte) (*Values, error) {
	v := Values{}
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// NewValuesFromFileInFS creates a new Values instance from a file in a file system.
func NewValuesFromFileInFS(f fs.FS, filename string) (*Values, error) {
	file, err := f.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return NewValuesFromYAML(data)
}

// NewValuesFromFile creates a new Values instance from a file.
func NewValuesFromFile(filename string) (*Values, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return NewValuesFromYAML(data)
}

func NewValuesFromFS(f fs.FS) (*Values, error) {
	return NewValuesFromFileInFS(f, "values.yaml")
}

func (v Values) Empty() bool {
	return len(v) == 0
}

// EqualYAML returns true if the two values are equal.
// The comparison is performed by converting both values
// to YAML and then comparing the YAML strings.
func (v Values) EqualYAML(other Values) bool {
	thisYaml, err := v.ToYAML()
	if err != nil {
		return false
	}

	otherYaml, err := other.ToYAML()
	if err != nil {
		return false
	}

	equal, err := yaml.EqualYAMLs(thisYaml, otherYaml)
	if err != nil {
		return false
	}

	return equal
}

func (c Values) ToYAML() ([]byte, error) {
	asJSON, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	asYAML, err := syaml.JSONToYAML(asJSON)
	if err != nil {
		return nil, err
	}

	return asYAML, nil
}

func (c Values) MustToYAML() []byte {
	asYAML, err := c.ToYAML()
	if err != nil {
		panic(err)
	}
	return asYAML
}

// ToJSON returns the JSON representation of the Values.
func (v Values) ToJSON() ([]byte, error) {
	return json.Marshal(v)
}

// ToJSONIndented returns the JSON representation of the Values with indentation.
func (v Values) ToJSONIndented() ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// DeepCopyInto copies the Values into another Values.
func (v *Values) DeepCopyInto(other *Values) {
	res := (&Values{}).Merge(v)
	*other = *res
}

// DeepCopy returns a deep copy of the Values.
func (v *Values) DeepCopy() *Values {
	if v == nil {
		return nil
	}
	other := Values{}
	v.DeepCopyInto(&other)
	return &other
}

////////////////////////////////////////////////////////////////////////////
// lookups
////////////////////////////////////////////////////////////////////////////

// Lookup returns the value associated with the given key.
// Keys can be nested using the "." character.
// Indexing is supported using the "[<index>]" syntax.
// Supported types are:
// - string
// - []string
// - int
// - []int
// - Values
// - []Values
// Examples:
// - "foo.bar" returns the value associated with the "bar" key in the "foo" map.
// - "foo[0].bar" returns the value associated with the "bar" key in the first element of the "foo" array.
func (v Values) Lookup(key string) (any, error) {
	if key == "" {
		return v, nil
	}

	// Split the key into components
	parts := strings.Split(key, ".")
	if len(parts) == 0 {
		return nil, ErrKeyNotFound
	}

	// Parse the first component for array indexing
	firstKey, index, err := parseIndex(parts[0])
	if err != nil {
		return nil, err
	}

	// Get the value for the first key
	value, exists := v[firstKey]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrKeyNotFound, firstKey)
	}

	// If we have an index, handle array/slice access
	if index >= 0 {
		value, err = getIndexedValue(value, index)
		if err != nil {
			return nil, err
		}
	}

	// If this was the last component, return the value
	if len(parts) == 1 {
		return value, nil
	}

	// Otherwise, continue with nested lookup
	restKey := strings.Join(parts[1:], ".")
	return lookupNested(value, restKey)
}

// getIndexedValue retrieves a value from an array/slice at the given index
func getIndexedValue(value interface{}, index int) (interface{}, error) {
	switch v := value.(type) {
	case []interface{}:
		if index >= len(v) {
			return nil, ErrIndexOutOfBounds
		}
		return v[index], nil
	case []string:
		if index >= len(v) {
			return nil, ErrIndexOutOfBounds
		}
		return v[index], nil
	case []int:
		if index >= len(v) {
			return nil, ErrIndexOutOfBounds
		}
		return v[index], nil
	case []Values:
		if index >= len(v) {
			return nil, ErrIndexOutOfBounds
		}
		return v[index], nil
	default:
		return nil, fmt.Errorf("%w: cannot index into %T", ErrInvalidType, value)
	}
}

// lookupNested continues lookup in a nested structure
func lookupNested(value interface{}, key string) (interface{}, error) {
	switch v := value.(type) {
	case Values:
		return v.Lookup(key)
	case map[string]interface{}:
		return Values(v).Lookup(key)
	case string, int, int32, int64, float32, float64, bool:
		// Terminal values cannot have nested lookups
		if key == "" {
			return v, nil
		}
		return nil, fmt.Errorf("%w: cannot lookup %s in %T", ErrKeyNotFound, key, v)
	default:
		if key == "" {
			return v, nil
		}
		return nil, fmt.Errorf("%w: %s", ErrKeyNotFound, key)
	}
}

// LookupFist is the same as Lookup() but tries several possible keys until one of them is found.
// It returns the value, the key where it was found and an error if the value.
func (v Values) LookupFirst(keys []string) (any, string, error) {
	for _, key := range keys {
		val, err := v.Lookup(key)
		if err == nil {
			return val, key, nil
		}
	}
	return "", "", fmt.Errorf("%w: one of %+v", ErrKeyNotFound, keys)
}

func (v Values) LookupString(key string) (string, error) {
	valAny, err := v.Lookup(key)
	if err != nil {
		return "", err
	}
	return toString(valAny)
}

func (v Values) LookupValues(key string) (Values, error) {
	valAny, err := v.Lookup(key)
	if err != nil {
		return nil, err
	}
	return toValues(valAny)
}

// LookupFirstString is the same as LookupString() but tries several possible keys
// until one of them is found
// It returns the value, the key where it was found and an error if the value.
func (v Values) LookupFirstString(keys []string) (string, string, error) {
	valAny, foundAt, err := v.LookupFirst(keys)
	if err != nil {
		return "", "", err
	}

	valStr, err := toString(valAny)
	if err != nil {
		return "", "", fmt.Errorf("key %s: %w", foundAt, err)
	}
	return valStr, foundAt, nil
}

func (v Values) LookupInt(key string) (int, error) {
	valAny, err := v.Lookup(key)
	if err != nil {
		return 0, err
	}
	return toInt(valAny)
}

// LookupFirstInt is the same as LookupInt() but tries several possible keys
// until one of them is found
// It returns the value, the key where it was found and an error if the value.
func (v Values) LookupFirstInt(keys []string) (int, string, error) {
	valAny, foundAt, err := v.LookupFirst(keys)
	if err != nil {
		return 0, "", err
	}

	valInt, err := toInt(valAny)
	if err != nil {
		return 0, "", fmt.Errorf("key %s: %w", foundAt, err)
	}
	return valInt, foundAt, nil
}

// Set sets the value at the given key path.
// Keys can be nested using the "." character.
// Indexing is supported using the "[<index>]" syntax.
// The function will create intermediate maps/slices as needed.
// Examples:
// - "foo.bar" sets the value in the "bar" key in the "foo" map
// - "foo[0].bar" sets the value in the "bar" key in the first element of the "foo" array
func (v Values) Set(key string, value interface{}) error {
	if key == "" {
		return fmt.Errorf("%w: empty key", ErrInvalidIndexUsage)
	}

	parts := strings.Split(key, ".")
	if len(parts) == 0 {
		return nil
	}

	firstKey, index, err := parseIndex(parts[0])
	if err != nil {
		return err
	}

	// If this is the final component, set the value directly
	if len(parts) == 1 {
		if index >= 0 {
			return v.setArrayValue(firstKey, index, value)
		}
		v[firstKey] = value
		return nil
	}

	// Handle intermediate nodes
	restKey := strings.Join(parts[1:], ".")

	if index >= 0 {
		return v.setNestedArrayValue(firstKey, index, restKey, value)
	}

	return v.setNestedMapValue(firstKey, restKey, value)
}

// setArrayValue sets a value in an array at the specified index
func (v Values) setArrayValue(key string, index int, value interface{}) error {
	existing := v[key]

	// Convert existing value to []interface{} if needed
	var arr []interface{}
	switch e := existing.(type) {
	case []interface{}:
		arr = e
	case nil:
		arr = make([]interface{}, 0)
	default:
		// If the existing value is not an array, we need to replace it
		arr = make([]interface{}, 0)
	}

	// Extend array if needed
	if index >= len(arr) {
		newArr := make([]interface{}, index+1)
		copy(newArr, arr)
		arr = newArr
	}

	arr[index] = value
	v[key] = arr
	return nil
}

// setNestedArrayValue sets a nested value through an array index
func (v Values) setNestedArrayValue(key string, index int, restKey string, value interface{}) error {
	existing := v[key]

	// Convert existing value to []interface{} if needed
	var arr []interface{}
	switch e := existing.(type) {
	case []interface{}:
		arr = e
	case nil:
		arr = make([]interface{}, index+1)
	default:
		arr = make([]interface{}, index+1)
	}

	// Extend array if needed
	if index >= len(arr) {
		newArr := make([]interface{}, index+1)
		copy(newArr, arr)
		arr = newArr
	}

	// Ensure we have a Values at this index
	if arr[index] == nil {
		arr[index] = make(Values)
	}

	nested, err := toValues(arr[index])
	if err != nil {
		// Create new Values if conversion fails
		nested = make(Values)
		arr[index] = nested
	}

	v[key] = arr
	return nested.Set(restKey, value)
}

// setNestedMapValue sets a nested value in a map
func (v Values) setNestedMapValue(key string, restKey string, value interface{}) error {
	existing := v[key]

	// Ensure we have a Values at this key
	if existing == nil {
		v[key] = make(Values)
		existing = v[key]
	}

	nested, err := toValues(existing)
	if err != nil {
		// Create new Values if conversion fails
		nested = make(Values)
		v[key] = nested
	}

	return nested.Set(restKey, value)
}

// Rebase rebases the Values on top a given base.
// The new base can be specified as a string of keys separated by the SplitToken.
// For example, if the Values is {"foo": {"bar": "baz"}} and
// - the base is "new", then the result is { "new": {"foo": {"bar": "baz"}}}.
// - the base is "new.base", then the result is {"new": { "base": {"foo": {"bar": "baz"}}}}.
func (v Values) Rebase(base string) *Values {
	comps := strings.Split(base, SplitToken)
	if len(comps) == 0 {
		return &v
	}

	this := comps[0]
	rest := strings.Join(comps[1:], SplitToken)

	if len(comps) == 1 {
		return &Values{this: v}
	}

	return &Values{this: *v.Rebase(rest)}
}

func parseIndex(s string) (string, int, error) {
	start := strings.Index(s, IndexOpenChar)
	end := strings.Index(s, IndexCloseChar)

	if start == -1 && end == -1 {
		return s, -1, nil
	}

	if (start != -1 && end == -1) || (start == -1 && end != -1) {
		return "", -1, ErrMalformedIndex
	}

	index, err := strconv.Atoi(s[start+1 : end])
	if err != nil {
		return "", -1, ErrMalformedIndex
	}

	// Reject negative indices
	if index < 0 {
		return "", -1, ErrMalformedIndex
	}

	return s[:start], index, nil
}

////////////////////////////////////////////////////////////////////////////
// merges
////////////////////////////////////////////////////////////////////////////

// mergeConfig is a configuration for the Merge() function.
// +k8s:deepcopy-gen=false
type mergeConfig struct {
	deepMergeSlice          bool
	overwriteWithEmptyValue bool
}

func newMergeConfig(opts ...MergeOption) *mergeConfig {
	c := &mergeConfig{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (m mergeConfig) toMergoOptions() []func(*mergo.Config) {
	opts := []func(*mergo.Config){}
	if m.deepMergeSlice {
		opts = append(opts, mergo.WithSliceDeepCopy)
	}
	if m.overwriteWithEmptyValue {
		opts = append(opts, mergo.WithOverwriteWithEmptyValue)
	}
	return opts
}

// +k8s:deepcopy-gen=false
type MergeOption func(*mergeConfig)

// WithMergeSlices is a merge option that tells the Merge() function to merge slices.
// By default, slices are just overwritten. So [1, 2] merged with [3, 4] gives [3, 4].
// With this option, [1, 2] merged with [3, 4] gives [1, 2, 3, 4].
func WithMergeSlices(c *mergeConfig) {
	c.deepMergeSlice = true
}

// WithOverwriteWithEmptyValue is a merge option that tells the Merge() function to overwrite values with empty values.
// By default, empty values are not merged. So {"foo": "bar"} merged with {"foo": ""} gives {"foo": "bar"}.
// With this option, {"foo": "bar"} merged with {"foo": ""} gives {"foo": ""}.
func WithOverwriteWithEmptyValue(c *mergeConfig) {
	c.overwriteWithEmptyValue = true
}

// Merge merges the given values into the current values, returning the new merged values.
func (v Values) Merge(other *Values, opts ...MergeOption) *Values {
	cfg := newMergeConfig(opts...)

	if v.Empty() {
		return other
	}
	if other.Empty() {
		return &v
	}

	// Create deep copies and normalize types
	thisNormalized := normalizeValues(v)
	otherNormalized := normalizeValues(*other)

	// Use mergo to merge the normalized values
	if err := mergo.MergeWithOverwrite(&thisNormalized, &otherNormalized, cfg.toMergoOptions()...); err != nil {
		// Fall back to YAML conversion if mergo fails
		return v.mergeViaYAML(other, cfg)
	}

	return &thisNormalized
}

// normalizeValues recursively normalizes all map[string]interface{} to Values
func normalizeValues(v Values) Values {
	result := make(Values)
	for key, value := range v {
		result[key] = normalizeValue(value)
	}
	return result
}

// normalizeValue recursively normalizes a single value
func normalizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		// Convert map[string]interface{} to Values
		normalized := make(Values)
		for k, v := range val {
			normalized[k] = normalizeValue(v)
		}
		return normalized
	case Values:
		// Recursively normalize Values
		return normalizeValues(val)
	case []interface{}:
		// Normalize arrays
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = normalizeValue(item)
		}
		return result
	default:
		// Return other types as-is
		return v
	}
}

// mergeViaYAML is the fallback merge method using YAML conversion
func (v Values) mergeViaYAML(other *Values, cfg *mergeConfig) *Values {
	thisYaml, err := v.ToYAML()
	if err != nil {
		return nil
	}
	otherYaml, err := other.ToYAML()
	if err != nil {
		return nil
	}

	thisValues, err := NewValuesFromYAML(thisYaml)
	if err != nil {
		return nil
	}
	otherValues, err := NewValuesFromYAML(otherYaml)
	if err != nil {
		return nil
	}

	if err := mergo.MergeWithOverwrite(thisValues, otherValues, cfg.toMergoOptions()...); err != nil {
		return nil
	}
	return thisValues
}
