package orchestrator

// Environment output-to-input piping (ADR-0008).
//
// A component spec may carry placeholders of the form ${component.outputs.field}.
// At create time, OPORD topologically orders components into waves so each
// component runs after every component it references. Inside each wave the
// components run concurrently; when the whole wave reaches "ready" their
// outputs (read from resources.observed) are available to the next wave's
// placeholder substitution.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// placeholderRe matches any occurrence of ${component.outputs.field}. Field
// supports dotted paths (e.g. connection.host).
var placeholderRe = regexp.MustCompile(`\$\{([a-zA-Z][a-zA-Z0-9_-]*)\.outputs\.([a-zA-Z0-9_][a-zA-Z0-9_.\-]*)\}`)

// wholeStringRe matches when the entire string IS a single placeholder. Used
// so a whole-string placeholder preserves its typed value (number, list, …)
// instead of being stringified.
var wholeStringRe = regexp.MustCompile(`^\$\{([a-zA-Z][a-zA-Z0-9_-]*)\.outputs\.([a-zA-Z0-9_][a-zA-Z0-9_.\-]*)\}$`)

// componentRefs returns the set of component names referenced by placeholders
// anywhere in spec. The result is sorted (deterministic).
func componentRefs(spec json.RawMessage) ([]string, error) {
	var v any
	if err := json.Unmarshal(spec, &v); err != nil {
		return nil, fmt.Errorf("componentRefs: %w", err)
	}
	seen := map[string]struct{}{}
	walkStrings(v, func(s string) {
		for _, m := range placeholderRe.FindAllStringSubmatch(s, -1) {
			seen[m[1]] = struct{}{}
		}
	})
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// walkStrings invokes fn on every string value within a JSON value tree.
func walkStrings(v any, fn func(string)) {
	switch x := v.(type) {
	case map[string]any:
		for _, vv := range x {
			walkStrings(vv, fn)
		}
	case []any:
		for _, vv := range x {
			walkStrings(vv, fn)
		}
	case string:
		fn(x)
	}
}

// substituteRefs walks spec, replacing every ${comp.outputs.field} with the
// concrete value from outputs[comp]. A whole-string placeholder preserves the
// referenced value's type (string, number, bool, list, object); an
// interpolated placeholder is stringified.
func substituteRefs(spec json.RawMessage, outputs map[string]map[string]any) (json.RawMessage, error) {
	if !placeholderRe.Match(spec) {
		return spec, nil
	}
	var v any
	if err := json.Unmarshal(spec, &v); err != nil {
		return nil, fmt.Errorf("substituteRefs unmarshal: %w", err)
	}
	sub, err := substValue(v, outputs)
	if err != nil {
		return nil, err
	}
	return json.Marshal(sub)
}

func substValue(v any, outputs map[string]map[string]any) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		for k, vv := range x {
			sub, err := substValue(vv, outputs)
			if err != nil {
				return nil, err
			}
			x[k] = sub
		}
		return x, nil
	case []any:
		for i, vv := range x {
			sub, err := substValue(vv, outputs)
			if err != nil {
				return nil, err
			}
			x[i] = sub
		}
		return x, nil
	case string:
		return substString(x, outputs)
	default:
		return v, nil
	}
}

func substString(s string, outputs map[string]map[string]any) (any, error) {
	if m := wholeStringRe.FindStringSubmatch(s); m != nil {
		return resolveOutput(m[1], m[2], outputs)
	}
	var lastErr error
	out := placeholderRe.ReplaceAllStringFunc(s, func(match string) string {
		m := placeholderRe.FindStringSubmatch(match)
		v, err := resolveOutput(m[1], m[2], outputs)
		if err != nil {
			lastErr = err
			return ""
		}
		return fmt.Sprint(v)
	})
	if lastErr != nil {
		return nil, lastErr
	}
	return out, nil
}

func resolveOutput(comp, field string, outputs map[string]map[string]any) (any, error) {
	o, ok := outputs[comp]
	if !ok {
		return nil, fmt.Errorf("placeholder ${%s.outputs.%s}: component %q has no outputs yet", comp, field, comp)
	}
	cur := any(o)
	for _, p := range strings.Split(field, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("placeholder ${%s.outputs.%s}: cannot traverse %q (parent is not an object)", comp, field, p)
		}
		cur, ok = m[p]
		if !ok {
			return nil, fmt.Errorf("placeholder ${%s.outputs.%s}: component %q has no output %q", comp, field, comp, field)
		}
	}
	return cur, nil
}

// componentWaves groups components into topological waves: wave 0 has every
// component with no dependencies; wave N has components whose deps are all in
// waves 0..N-1. Each wave is sorted (deterministic) and may run concurrently.
// Errors out on unknown refs, self-references, and dependency cycles before
// any provisioning starts.
func componentWaves(deps map[string][]string, all []string) ([][]string, error) {
	nodes := map[string]bool{}
	for _, n := range all {
		nodes[n] = true
	}
	inDeg := map[string]int{}
	forward := map[string][]string{}
	for _, n := range all {
		inDeg[n] = 0
	}
	for n, ds := range deps {
		if !nodes[n] {
			continue
		}
		for _, d := range ds {
			if d == n {
				return nil, fmt.Errorf("component %q references itself", n)
			}
			if !nodes[d] {
				return nil, fmt.Errorf("component %q references unknown component %q", n, d)
			}
			forward[d] = append(forward[d], n)
			inDeg[n]++
		}
	}
	var waves [][]string
	remaining := len(all)
	for remaining > 0 {
		var wave []string
		for _, n := range all {
			if inDeg[n] == 0 {
				wave = append(wave, n)
			}
		}
		if len(wave) == 0 {
			return nil, fmt.Errorf("dependency cycle detected among components: %s", strings.Join(unresolved(inDeg), ", "))
		}
		for _, n := range wave {
			inDeg[n] = -1
			for _, m := range forward[n] {
				inDeg[m]--
			}
		}
		sort.Strings(wave)
		waves = append(waves, wave)
		remaining -= len(wave)
	}
	return waves, nil
}

// unresolved returns the still-unscheduled nodes (used in cycle error messages).
func unresolved(inDeg map[string]int) []string {
	out := make([]string, 0, len(inDeg))
	for n, d := range inDeg {
		if d >= 0 {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}
