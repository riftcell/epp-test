package runner

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Scenario is the top-level structure of a YAML scenario file.
// It describes an ordered set of EPP/RRI operations and optional
// registrar-specific overrides.
type Scenario struct {
	Name      string                       `yaml:"name"`
	RFC       string                       `yaml:"rfc,omitempty"`
	Matrix    []string                     `yaml:"matrix,omitempty"`
	Steps     []Step                       `yaml:"steps"`
	Overrides map[string]RegistrarOverride `yaml:"overrides,omitempty"`
}

// Step is a single operation within a scenario.
type Step struct {
	Name   string         `yaml:"name"`
	Op     string         `yaml:"op"`
	Params map[string]any `yaml:"params,omitempty"`
	Expect *Expect        `yaml:"expect,omitempty"`
}

// Expect defines the assertion criteria for a step response.
type Expect struct {
	Code   int            `yaml:"code,omitempty"`
	Fields map[string]any `yaml:"fields,omitempty"`
}

// RegistrarOverride provides registrar-specific step overrides within a scenario.
// The runner deep-merges each StepOverride's params onto the base step params
// before executing the step (CONTEXT.md D-07).
type RegistrarOverride struct {
	Steps map[string]StepOverride `yaml:"steps,omitempty"`
}

// StepOverride holds the per-registrar param overrides for a named step.
type StepOverride struct {
	Params map[string]any `yaml:"params,omitempty"`
}

// LoadScenario reads and parses the YAML scenario file at path.
// Any I/O or parse error is wrapped with the path for diagnostic clarity.
func LoadScenario(path string) (*Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load scenario %s: %w", path, err)
	}
	var s Scenario
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse scenario %s: %w", path, err)
	}
	return &s, nil
}

// deepMerge returns a new map that combines base and override.
// Keys present only in base or only in override appear as-is.
// When both base[k] and override[k] are map[string]any, they are
// merged recursively. For all other types, override[k] replaces base[k].
//
// Neither input map is mutated.
func deepMerge(base, override map[string]any) map[string]any {
	merged := make(map[string]any, len(base))
	for k, v := range base {
		merged[k] = v
	}
	for k, ov := range override {
		bv, exists := merged[k]
		if exists {
			bMap, bOk := bv.(map[string]any)
			oMap, oOk := ov.(map[string]any)
			if bOk && oOk {
				merged[k] = deepMerge(bMap, oMap)
				continue
			}
		}
		merged[k] = ov
	}
	return merged
}
