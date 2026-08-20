package runner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/riftcell/epp-test/pkg/registrar"
)

// RunScenario executes the YAML scenario at path against the given Registrar.
//
// Execution order:
//  1. Load and parse the scenario file.
//  2. Register a single t.Cleanup (before any steps) that iterates cleanupStack
//     in reverse order — so created objects are deleted in reverse creation order
//     (Pitfall 3: register cleanup BEFORE first step, not after).
//  3. For each step: merge registrar-specific overrides (deepMerge), interpolate
//     ${step.field} references, inject run-ID prefix on create_* ops, dispatch
//     to opTable or execWaitForPoll, assert expectations, capture results, register
//     cleanup closures for successful create_* ops.
//  4. Append a ScenarioResult to the package-level collector (D-08).
func RunScenario(t testing.TB, path string, reg registrar.Registrar) {
	t.Helper()

	scenario, err := LoadScenario(path)
	require.NoError(t, err, "RunScenario: load scenario %s", path)

	var (
		cleanupStack   []func(context.Context)
		captured       = map[string]map[string]any{}
		prefix         = runID()
		start          = time.Now()
		stepResults    []StepResult
		scenarioPassed = true
	)

	// Register a single cleanup BEFORE any step executes (Pitfall 3).
	// The closure captures the slice header via the local variable; appends to the
	// slice after this point are visible at call time because Go slices are reference-
	// typed through the pointer captured by the closure.
	t.Cleanup(func() {
		ctx := context.Background()
		for i := len(cleanupStack) - 1; i >= 0; i-- {
			cleanupStack[i](ctx)
		}
	})

	// Look up the registrar-specific override block (may be zero-value if not present).
	ov := scenario.Overrides[reg.Name()]

	ctx := context.Background()

	for _, step := range scenario.Steps {
		// Merge registrar-specific override params onto base step params (D-07).
		params := step.Params
		if params == nil {
			params = map[string]any{}
		}
		if ovStep, ok := ov.Steps[step.Name]; ok && ovStep.Params != nil {
			params = deepMerge(params, ovStep.Params)
		}

		// Interpolate ${step.field} references in string param values (D-03).
		params = interpolateParams(params, captured)

		// Inject run-ID prefix on create_* op name and id params (YAML-07).
		params = injectRunIDPrefix(step.Op, params, prefix)

		stepStart := time.Now()
		var (
			result  any
			stepErr error
		)

		// Dispatch: wait_for_poll is handled specially; all other ops via opTable.
		if step.Op == "wait_for_poll" {
			result, stepErr = execWaitForPoll(ctx, reg, params)
		} else {
			fn, ok := opTable[step.Op]
			if !ok {
				stepErr = fmt.Errorf("unknown op %q", step.Op)
			} else {
				result, stepErr = fn(ctx, reg, params)
			}
		}

		elapsed := time.Since(stepStart)

		// Assert expectations.
		passed, assertErr := assertExpect(step.Expect, result, stepErr)
		if !passed {
			if assertErr != nil {
				t.Errorf("step %q (%s): %v", step.Name, step.Op, assertErr)
			}
			stepResults = append(stepResults, StepResult{
				Name:    step.Name,
				Op:      step.Op,
				Passed:  false,
				Elapsed: elapsed,
				Err:     errMsg(assertErr, stepErr),
			})
			// Append to collector before returning so the result is always recorded.
			Append(ScenarioResult{
				Name:      scenario.Name,
				Registrar: reg.Name(),
				File:      path,
				RFC:       scenario.RFC,
				Passed:    false,
				Elapsed:   time.Since(start),
				Steps:     stepResults,
			})
			// Stop execution on first failure so later steps don't run on broken state.
			return
		}

		// Capture result for variable interpolation in later steps.
		// For check_* ops that return slices, capture index 0 (Pitfall 2).
		captureVal := singleResult(result)
		if captureVal != nil {
			captured[step.Name] = captureResult(captureVal)
		}

		// Register cleanup for successful create_* ops.
		registerCleanup(t, reg, step.Op, params, result, &cleanupStack)

		stepResults = append(stepResults, StepResult{
			Name:    step.Name,
			Op:      step.Op,
			Passed:  true,
			Elapsed: elapsed,
		})
	}

	Append(ScenarioResult{
		Name:      scenario.Name,
		Registrar: reg.Name(),
		File:      path,
		RFC:       scenario.RFC,
		Passed:    scenarioPassed,
		Elapsed:   time.Since(start),
		Steps:     stepResults,
	})
}

// RunMatrix runs the scenario at path against every registrar in regs whose name
// appears in the scenario's Matrix list (YAML-04 / D-06). An empty or absent Matrix
// means "run against all registrars in regs". Each matching registrar runs in its own
// t.Run subtest named by the registrar key, producing one independent ScenarioResult.
// Matrix entries with no corresponding registrar in regs are skipped silently.
func RunMatrix(t *testing.T, path string, regs map[string]registrar.Registrar) {
	t.Helper()

	scenario, err := LoadScenario(path)
	require.NoError(t, err, "RunMatrix: load scenario %s", path)

	// Determine which registrar names to run. Empty matrix => all regs.
	var names []string
	if len(scenario.Matrix) == 0 {
		for name := range regs {
			names = append(names, name)
		}
		sort.Strings(names) // deterministic subtest order
	} else {
		names = scenario.Matrix
	}

	for _, name := range names {
		reg, ok := regs[name]
		if !ok {
			// Matrix declares a registrar not provided by the caller — skip silently.
			continue
		}
		t.Run(name, func(t *testing.T) {
			RunScenario(t, path, reg)
		})
	}
}

// RunDir runs all *.yaml scenario files in dir as individual t.Run subtests.
// Each subtest is named after the filename without the .yaml extension (Pattern 11).
func RunDir(t *testing.T, dir string, reg registrar.Registrar) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	require.NoError(t, err, "RunDir: glob %s", dir)
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".yaml")
		t.Run(name, func(t *testing.T) {
			RunScenario(t, f, reg)
		})
	}
}

// assertExpect checks a step's Expect criteria against the actual result and error.
// Returns (true, nil) on pass, (false, descriptive error) on failure.
func assertExpect(expect *Expect, result any, stepErr error) (bool, error) {
	if expect == nil {
		// No expectations — pass unconditionally (but propagate unexpected errors).
		if stepErr != nil {
			var eppErr *registrar.EPPError
			if errors.As(stepErr, &eppErr) {
				// An EPPError with no expectation is treated as a pass if 1xxx.
				if eppErr.Code >= 1000 && eppErr.Code < 2000 {
					return true, nil
				}
				return false, fmt.Errorf("EPP error (code %d): %w", eppErr.Code, stepErr)
			}
			return false, fmt.Errorf("unexpected error: %w", stepErr)
		}
		return true, nil
	}

	// Determine the actual EPP result code.
	actualCode := 0
	if stepErr == nil {
		actualCode = 1000 // success — no error means 1000
	} else {
		var eppErr *registrar.EPPError
		if errors.As(stepErr, &eppErr) {
			actualCode = eppErr.Code
		} else {
			// Non-EPP error (network, decode, etc.) — always a failure.
			return false, fmt.Errorf("unexpected non-EPP error: %w", stepErr)
		}
	}

	// Check result code.
	if expect.Code != 0 && actualCode != expect.Code {
		return false, fmt.Errorf("expected EPP code %d, got %d (err: %w)", expect.Code, actualCode, stepErr)
	}

	// If code matched a success code but there's still an error, that's unexpected.
	if expect.Code >= 1000 && expect.Code < 2000 && stepErr != nil {
		// Only fail if it's a non-EPP error; EPP errors are already handled above.
		var eppErr *registrar.EPPError
		if !errors.As(stepErr, &eppErr) {
			return false, fmt.Errorf("unexpected error on success code: %w", stepErr)
		}
	}

	// Check expected fields (D-02).
	if len(expect.Fields) > 0 {
		capturedFields := captureResult(singleResult(result))
		for wantKey, wantVal := range expect.Fields {
			// Case-insensitive key lookup.
			actualVal, ok := findField(capturedFields, wantKey)
			if !ok {
				return false, fmt.Errorf("field %q not found in result (available: %v)", wantKey, fieldNames(capturedFields))
			}
			wantStr := fmt.Sprintf("%v", wantVal)
			actualStr := fmt.Sprintf("%v", actualVal)
			if wantStr != actualStr {
				return false, fmt.Errorf("field %q: expected %q, got %q", wantKey, wantStr, actualStr)
			}
		}
	}

	return true, nil
}

// findField looks up wantKey in m using case-insensitive comparison.
// Returns the value and true if found.
func findField(m map[string]any, wantKey string) (any, bool) {
	lower := strings.ToLower(wantKey)
	for k, v := range m {
		if strings.ToLower(k) == lower {
			return v, true
		}
	}
	return nil, false
}

// fieldNames returns a sorted list of key names from a map (for error messages).
func fieldNames(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// singleResult returns the first element of a []DomainResult, []ContactResult, or
// []HostResult slice, or returns the value itself for non-slice results.
// Returns nil for nil input or empty slices.
func singleResult(result any) any {
	if result == nil {
		return nil
	}
	switch vt := result.(type) {
	case []registrar.DomainResult:
		if len(vt) == 0 {
			return nil
		}
		return vt[0]
	case []registrar.ContactResult:
		if len(vt) == 0 {
			return nil
		}
		return vt[0]
	case []registrar.HostResult:
		if len(vt) == 0 {
			return nil
		}
		return vt[0]
	}
	return result
}

// registerCleanup appends a cleanup function for successful create_* operations.
// The cleanup calls the corresponding delete method with the created object's name or ID.
func registerCleanup(t testing.TB, reg registrar.Registrar, op string, params map[string]any, result any, stack *[]func(context.Context)) {
	t.Helper()
	switch op {
	case "create_domain":
		name := stringParam(params, "name")
		if dr, ok := result.(registrar.DomainResult); ok && dr.Name != "" {
			name = dr.Name
		}
		if name == "" {
			return
		}
		*stack = append(*stack, func(ctx context.Context) {
			if err := reg.DeleteDomain(ctx, name); err != nil {
				t.Logf("cleanup: delete domain %s: %v", name, err)
			}
		})

	case "create_contact":
		id := stringParam(params, "id")
		if cr, ok := result.(registrar.ContactResult); ok && cr.ID != "" {
			id = cr.ID
		}
		if id == "" {
			return
		}
		*stack = append(*stack, func(ctx context.Context) {
			if err := reg.DeleteContact(ctx, id); err != nil {
				t.Logf("cleanup: delete contact %s: %v", id, err)
			}
		})

	case "create_host":
		name := stringParam(params, "name")
		if hr, ok := result.(registrar.HostResult); ok && hr.Name != "" {
			name = hr.Name
		}
		if name == "" {
			return
		}
		*stack = append(*stack, func(ctx context.Context) {
			if err := reg.DeleteHost(ctx, name); err != nil {
				t.Logf("cleanup: delete host %s: %v", name, err)
			}
		})
	}
}

// execWaitForPoll polls the registrar until a matching message arrives or the timeout elapses.
// YAML params:
//
//	timeout:    duration string, default "30s"
//	match_type: optional PollMessage.Type to match
func execWaitForPoll(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	timeoutStr := stringParam(params, "timeout")
	if timeoutStr == "" {
		timeoutStr = "30s"
	}
	d, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("wait_for_poll: invalid timeout %q: %w", timeoutStr, err)
	}

	matchType := stringParam(params, "match_type")
	deadline := time.Now().Add(d)

	for time.Now().Before(deadline) {
		msg, err := reg.PollRead(ctx)
		if err != nil {
			// EPPError 1300 = queue empty — retry.
			var eppErr *registrar.EPPError
			if errors.As(err, &eppErr) && eppErr.Code == 1300 {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("wait_for_poll: PollRead error: %w", err)
		}

		// Message arrived. Check match_type if specified.
		if matchType == "" || msg.Type == matchType {
			if ackErr := reg.PollAck(ctx, msg.ID); ackErr != nil {
				return nil, fmt.Errorf("wait_for_poll: PollAck error: %w", ackErr)
			}
			return msg, nil
		}

		// Message type doesn't match — ack it and keep polling.
		_ = reg.PollAck(ctx, msg.ID) //nolint:errcheck // best-effort ack of non-matching message; failure will surface on next PollRead
	}

	return nil, fmt.Errorf("wait_for_poll: timeout after %s", d)
}

// errMsg returns a non-empty error string from the first non-nil error.
func errMsg(errs ...error) string {
	for _, e := range errs {
		if e != nil {
			return e.Error()
		}
	}
	return ""
}
