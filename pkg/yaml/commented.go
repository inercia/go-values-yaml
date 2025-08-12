package yaml

import (
	"bytes"
	"sort"
	"strings"

	syaml "sigs.k8s.io/yaml"
)

// CommentedOut serializes the provided full structure to YAML, but comments out
// any branches that are absent (or set to nil) in the masked structure.
//
// Typical usage is to pass the same structure twice: the first argument contains
// all fields with their desired values, and the second argument is the same
// structure but with some fields either omitted or set to nil wherever those
// fields should be commented out in the output.
//
// The function supports nested maps and lists. For maps, commenting can be
// applied selectively per key. For lists and scalars, commenting is applied to
// the entire value of the key when marked. Empty maps are rendered as `{}` and
// empty lists as `[]` consistent with sigs.k8s.io/yaml formatting.
func CommentedOut(full any, masked any) ([]byte, error) {
	// Normalize inputs to map[string]any recursively when possible.
	fn := normalizeToStringKeyed(full)
	mn := normalizeToStringKeyed(masked)

	var buf bytes.Buffer

	// If root is a map, emit keys deterministically.
	if fm, ok := fn.(map[string]any); ok {
		mm, _ := mn.(map[string]any)
		if err := emitMap(&buf, 0, fm, mm, false); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	// For non-map roots, render the entire document as one block, commented if
	// masked is nil.
	comment := mn == nil
	b, err := syaml.Marshal(fn)
	if err != nil {
		return nil, err
	}
	writeIndentedBlock(&buf, 0, string(b), comment)
	return buf.Bytes(), nil
}

func emitMap(buf *bytes.Buffer, indent int, fm map[string]any, mm map[string]any, parentComment bool) error {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(fm))
	for k := range fm {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fv := fm[k]
		mv, present := mm[k]
		childComment := parentComment || !present || mv == nil

		// If we need to comment the entire subtree for this key, render it as a
		// standalone YAML block and prefix each line with comment and indentation.
		if childComment {
			if err := emitKeyAsBlock(buf, indent, k, fv, true); err != nil {
				return err
			}
			continue
		}

		// Otherwise, render normally. For scalars and lists, we can render the
		// whole key as a block. For maps, we may need to selectively comment
		// nested keys, so handle non-empty maps manually.
		switch fvt := normalizeToStringKeyed(fv).(type) {
		case map[string]any:
			// If empty map, render inline as {} using YAML marshaller.
			if len(fvt) == 0 {
				if err := emitKeyAsBlock(buf, indent, k, fvt, false); err != nil {
					return err
				}
				continue
			}
			// Non-empty map: print "key:" then nested entries.
			writeLine(buf, indent, false, k+":")
			mvMap, _ := normalizeToStringKeyed(mv).(map[string]any)
			if err := emitMap(buf, indent+2, fvt, mvMap, false); err != nil {
				return err
			}
		default:
			// Scalars and lists can be rendered as a whole using YAML.
			if err := emitKeyAsBlock(buf, indent, k, fvt, false); err != nil {
				return err
			}
		}
	}
	return nil
}

// emitKeyAsBlock marshals a single-key map {key: value} using YAML, then emits
// the resulting lines with the provided indentation and optional comment prefix.
func emitKeyAsBlock(buf *bytes.Buffer, indent int, key string, value any, comment bool) error {
	// For commented scalars (or nil), emit a single-line "# key: value" to avoid
	// unnecessary quoting of simple keys (e.g., y, n, on) by the YAML marshaller.
	if comment {
		vn := normalizeToStringKeyed(value)
		if vn == nil || isScalar(vn) {
			vb, err := syaml.Marshal(vn)
			if err != nil {
				return err
			}
			vline := strings.TrimSpace(string(vb))
			// Quote keys that YAML might interpret as booleans or special values
			keyOut := key
			if needsQuoting(key) {
				keyOut = "\"" + key + "\""
			}
			writeLine(buf, indent, true, keyOut+": "+vline)
			return nil
		}
	}
	m := map[string]any{key: value}
	b, err := syaml.Marshal(m)
	if err != nil {
		return err
	}
	writeIndentedBlock(buf, indent, string(b), comment)
	return nil
}

func writeLine(buf *bytes.Buffer, indent int, comment bool, line string) {
	if comment {
		buf.WriteString(strings.Repeat(" ", indent))
		buf.WriteString("# ")
		buf.WriteString(line)
		buf.WriteByte('\n')
		return
	}
	buf.WriteString(strings.Repeat(" ", indent))
	buf.WriteString(line)
	buf.WriteByte('\n')
}

func writeIndentedBlock(buf *bytes.Buffer, indent int, block string, comment bool) {
	lines := strings.Split(strings.TrimRight(block, "\n"), "\n")
	prefix := strings.Repeat(" ", indent)
	for _, ln := range lines {
		if comment {
			buf.WriteString(prefix)
			buf.WriteString("# ")
			buf.WriteString(ln)
			buf.WriteByte('\n')
		} else {
			buf.WriteString(prefix)
			buf.WriteString(ln)
			buf.WriteByte('\n')
		}
	}
}

// normalizeToStringKeyed converts known YAML-like structures recursively so that
// all maps have string keys (map[string]any). Other values are returned as-is.
func normalizeToStringKeyed(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m2 := make(map[string]any, len(t))
		for k, vv := range t {
			m2[k] = normalizeToStringKeyed(vv)
		}
		return m2
	case map[any]any:
		m2 := make(map[string]any, len(t))
		for k, vv := range t {
			m2[asString(k)] = normalizeToStringKeyed(vv)
		}
		return m2
	case []any:
		l2 := make([]any, len(t))
		for i := range t {
			l2[i] = normalizeToStringKeyed(t[i])
		}
		return l2
	default:
		return v
	}
}

func asString(k any) string {
	switch s := k.(type) {
	case string:
		return s
	default:
		return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(toString(k), "\n", " "), "\r", " "), "\t", " "))
	}
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return strings.TrimSpace(string(must(syaml.Marshal(t))))
	}
}

func must(b []byte, _ error) []byte { return b }

func needsQuoting(str string) bool {
	s := strings.TrimSpace(str)
	if s == "" {
		return true
	}
	switch strings.ToLower(s) {
	case "y", "yes", "n", "no", "true", "false", "on", "off", "null", "~":
		return true
	}
	return strings.ContainsAny(s, ":#{}[]&,*>!|%@`\"\n\r\t")
}
