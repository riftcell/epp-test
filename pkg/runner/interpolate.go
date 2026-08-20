package runner

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// refPattern matches ${step_name.field_name} variable references.
// Group 1 = step name, group 2 = field name.
var refPattern = regexp.MustCompile(`\$\{([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)\}`)

// interpolateString replaces all ${step.field} references in s with the
// corresponding values from captured. If a reference cannot be resolved
// (missing step or field), the original token is left unchanged rather than
// silently becoming an empty string (RESEARCH Pitfall 2).
func interpolateString(s string, captured map[string]map[string]any) string {
	return refPattern.ReplaceAllStringFunc(s, func(match string) string {
		sub := refPattern.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		stepName, fieldName := sub[1], sub[2]
		fields, ok := captured[stepName]
		if !ok {
			return match // step not captured yet — leave token visible
		}
		val, ok := fields[fieldName]
		if !ok {
			return match // field not present — leave token visible
		}
		return fmt.Sprintf("%v", val)
	})
}

// interpolateParams returns a new map with all string values (and string
// elements within []any slices) interpolated via interpolateString.
// The input map is never mutated.
func interpolateParams(params map[string]any, captured map[string]map[string]any) map[string]any {
	out := make(map[string]any, len(params))
	for k, v := range params {
		switch vt := v.(type) {
		case string:
			out[k] = interpolateString(vt, captured)
		case []any:
			replaced := make([]any, len(vt))
			for i, elem := range vt {
				if s, ok := elem.(string); ok {
					replaced[i] = interpolateString(s, captured)
				} else {
					replaced[i] = elem
				}
			}
			out[k] = replaced
		default:
			out[k] = v
		}
	}
	return out
}

// captureResult converts a result struct (e.g., registrar.DomainResult) into
// a map[string]any keyed by the exported Go field names. The conversion is done
// via JSON marshal→unmarshal so no per-type switch is needed and the keys match
// the Go exported field names (e.g., "ID", "ROID", "Name"). On error an empty
// non-nil map is returned (capture is best-effort).
func captureResult(result any) map[string]any {
	data, err := json.Marshal(result)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]any{}
	}
	return m
}

// runID returns the value of the RUN_ID environment variable when set, or
// generates an 8-hex-character string from crypto/rand.
func runID() string {
	if id := os.Getenv("RUN_ID"); id != "" {
		return id
	}
	b := make([]byte, 4)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "fallback"
	}
	return fmt.Sprintf("%x", b)
}

// injectRunIDPrefix prepends prefix+"-" to the "name" and "id" params of
// create_* operations, unless the value is already a variable reference
// (starts with "${"). For all other ops the params are returned unchanged.
// The input map is never mutated; a new map is always returned.
func injectRunIDPrefix(op string, params map[string]any, prefix string) map[string]any {
	if !strings.HasPrefix(op, "create_") {
		return params
	}
	out := make(map[string]any, len(params))
	for k, v := range params {
		out[k] = v
	}
	for _, key := range []string{"name", "id"} {
		if val, ok := out[key]; ok {
			if s, ok := val.(string); ok && !strings.HasPrefix(s, "${") {
				out[key] = prefix + "-" + s
			}
		}
	}
	return out
}
