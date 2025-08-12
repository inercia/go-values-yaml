package yaml

import (
	"reflect"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pmezard/go-difflib/difflib"
	"sigs.k8s.io/yaml"
)

// EqualYAMLs compares two YAML documents by unmarshalling them and comparing the resulting objects.
// Note well that this function does not take into account spaces and comments: it only
// compares the contents.
func EqualYAMLs(a []byte, b []byte) (bool, error) {
	var err error

	var aYAMLBytes []byte
	if len(a) > 0 {
		var aYAML any
		if err := yaml.Unmarshal(a, &aYAML); err != nil {
			return false, err
		}
		// now serialize both objects and compare the resulting YAML
		aYAMLBytes, err = yaml.Marshal(aYAML)
		if err != nil {
			return false, err
		}
	}

	var bYAMLBytes []byte
	if len(b) > 0 {
		var bYAML any
		if err := yaml.Unmarshal(b, &bYAML); err != nil {
			return false, err
		}

		bYAMLBytes, err = yaml.Marshal(bYAML)
		if err != nil {
			return false, err
		}
	}

	return string(aYAMLBytes) == string(bYAMLBytes), nil
}

/////////////////////////////////////////////////////////////////////////////////////

var spewConfig = spew.ConfigState{
	Indent:                  " ",
	DisablePointerAddresses: true,
	DisableCapacities:       true,
	SortKeys:                true,
	DisableMethods:          true,
	MaxDepth:                10,
}

var spewConfigStringerEnabled = spew.ConfigState{
	Indent:                  " ",
	DisablePointerAddresses: true,
	DisableCapacities:       true,
	SortKeys:                true,
	MaxDepth:                10,
}

func typeAndKind(v interface{}) (reflect.Type, reflect.Kind) {
	t := reflect.TypeOf(v)
	k := t.Kind()

	if k == reflect.Ptr {
		t = t.Elem()
		k = t.Kind()
	}
	return t, k
}

// DiffYAML compares two YAML documents by unmarshalling them and comparing
// the resulting objects.
// The difference is returned as a string.
func DiffYAML(a []byte, b []byte) string {
	var err error

	var aYAMLBytes []byte
	if len(a) > 0 {
		var aYAML any
		if err := yaml.Unmarshal(a, &aYAML); err != nil {
			return ""
		}
		// now serialize both objects and compare the resulting YAML
		aYAMLBytes, err = yaml.Marshal(aYAML)
		if err != nil {
			return ""
		}
	}

	var bYAMLBytes []byte
	if len(b) > 0 {
		var bYAML any
		if err := yaml.Unmarshal(b, &bYAML); err != nil {
			return ""
		}

		bYAMLBytes, err = yaml.Marshal(bYAML)
		if err != nil {
			return ""
		}
	}

	return Diff(string(aYAMLBytes), string(bYAMLBytes))
}

func Diff(previous interface{}, actual interface{}) string {
	return DiffWithDescription(previous, "Previous", actual, "Actual")

}

func DiffWithDescription(previous interface{}, previousStr string, actual interface{}, actualStr string) string {
	if previous == nil || actual == nil {
		return ""
	}

	et, ek := typeAndKind(previous)
	at, _ := typeAndKind(actual)

	if et != at {
		return ""
	}

	if ek != reflect.Struct && ek != reflect.Map && ek != reflect.Slice && ek != reflect.Array && ek != reflect.String {
		return ""
	}

	var e, a string

	switch et {
	case reflect.TypeOf(""):
		e = reflect.ValueOf(previous).String()
		a = reflect.ValueOf(actual).String()
	case reflect.TypeOf(time.Time{}):
		e = spewConfigStringerEnabled.Sdump(previous)
		a = spewConfigStringerEnabled.Sdump(actual)
	default:
		e = spewConfig.Sdump(previous)
		a = spewConfig.Sdump(actual)
	}

	diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(e),
		B:        difflib.SplitLines(a),
		FromFile: previousStr,
		FromDate: "",
		ToFile:   actualStr,
		ToDate:   "",
		Context:  1,
	})

	return diff
}
