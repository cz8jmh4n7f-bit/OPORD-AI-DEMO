package orchestrator

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestComponentRefs(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want []string
	}{
		{"none", `{"region":"eu-central-1"}`, []string{}},
		{"single", `{"db":"${postgres.outputs.endpoint}"}`, []string{"postgres"}},
		{"multiple", `{"db":"${postgres.outputs.endpoint}","queue":"${jobs.outputs.queue_url}"}`, []string{"jobs", "postgres"}},
		{"nested object", `{"env":{"DB_HOST":"${postgres.outputs.endpoint}"}}`, []string{"postgres"}},
		{"interpolation", `{"url":"redis://${cache.outputs.host}:6379"}`, []string{"cache"}},
		{"duplicate refs to same component", `{"a":"${x.outputs.foo}","b":"${x.outputs.bar}"}`, []string{"x"}},
		{"in array", `{"hosts":["${a.outputs.h}","${b.outputs.h}"]}`, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := componentRefs([]byte(tt.spec))
			if err != nil {
				t.Fatalf("componentRefs: %v", err)
			}
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// Whole-string placeholders preserve type.
func TestSubstituteRefs_WholeStringTyped(t *testing.T) {
	outs := map[string]map[string]any{
		"db": {
			"host": "db.example.com",
			"port": float64(5432),
			"ips":  []any{"10.0.0.1", "10.0.0.2"},
		},
	}

	cases := []struct {
		spec string
		path string
		want any
	}{
		{`{"endpoint":"${db.outputs.host}"}`, "endpoint", "db.example.com"},
		{`{"port":"${db.outputs.port}"}`, "port", float64(5432)},
		{`{"ips":"${db.outputs.ips}"}`, "ips", []any{"10.0.0.1", "10.0.0.2"}},
	}
	for _, tc := range cases {
		got, err := substituteRefs(json.RawMessage(tc.spec), outs)
		if err != nil {
			t.Fatalf("substituteRefs(%s): %v", tc.spec, err)
		}
		var v map[string]any
		_ = json.Unmarshal(got, &v)
		if !reflect.DeepEqual(v[tc.path], tc.want) {
			t.Errorf("spec=%s: got %v (%T), want %v (%T)", tc.spec, v[tc.path], v[tc.path], tc.want, tc.want)
		}
	}
}

// Inline interpolation always produces a string.
func TestSubstituteRefs_Interpolation(t *testing.T) {
	outs := map[string]map[string]any{
		"cache": {"host": "cache.example.com", "port": float64(6379)},
	}
	spec := json.RawMessage(`{"url":"redis://${cache.outputs.host}:${cache.outputs.port}"}`)
	got, err := substituteRefs(spec, outs)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	_ = json.Unmarshal(got, &v)
	if v["url"] != "redis://cache.example.com:6379" {
		t.Errorf("got %v", v["url"])
	}
}

// Nested fields (dotted) work.
func TestSubstituteRefs_NestedField(t *testing.T) {
	outs := map[string]map[string]any{
		"db": {"connection": map[string]any{"host": "h", "port": float64(5432)}},
	}
	got, err := substituteRefs(json.RawMessage(`{"h":"${db.outputs.connection.host}"}`), outs)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	_ = json.Unmarshal(got, &v)
	if v["h"] != "h" {
		t.Errorf("got %v", v["h"])
	}
}

func TestSubstituteRefs_Errors(t *testing.T) {
	cases := []struct {
		name string
		spec string
		outs map[string]map[string]any
		msg  string
	}{
		{"missing component", `{"x":"${nope.outputs.foo}"}`, map[string]map[string]any{}, "no outputs yet"},
		{"missing field", `{"x":"${db.outputs.nope}"}`, map[string]map[string]any{"db": {"host": "h"}}, "has no output"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := substituteRefs(json.RawMessage(tc.spec), tc.outs)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.msg) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.msg)
			}
		})
	}
}

func TestSubstituteRefs_NoPlaceholders(t *testing.T) {
	spec := json.RawMessage(`{"region":"eu-central-1","x":42}`)
	got, err := substituteRefs(spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(spec) {
		t.Errorf("got %s, want %s (fast path should return input)", got, spec)
	}
}

func TestComponentWaves(t *testing.T) {
	tests := []struct {
		name string
		deps map[string][]string
		all  []string
		want [][]string
	}{
		{
			name: "no deps - one wave",
			all:  []string{"a", "b", "c"},
			want: [][]string{{"a", "b", "c"}},
		},
		{
			name: "linear chain",
			deps: map[string][]string{"b": {"a"}, "c": {"b"}},
			all:  []string{"a", "b", "c"},
			want: [][]string{{"a"}, {"b"}, {"c"}},
		},
		{
			name: "parallel then merge",
			deps: map[string][]string{"merged": {"a", "b"}},
			all:  []string{"a", "b", "merged"},
			want: [][]string{{"a", "b"}, {"merged"}},
		},
		{
			name: "diamond",
			deps: map[string][]string{"b": {"a"}, "c": {"a"}, "d": {"b", "c"}},
			all:  []string{"a", "b", "c", "d"},
			want: [][]string{{"a"}, {"b", "c"}, {"d"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := componentWaves(tt.deps, tt.all)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComponentWaves_Errors(t *testing.T) {
	cases := []struct {
		name string
		deps map[string][]string
		all  []string
		msg  string
	}{
		{"simple cycle", map[string][]string{"a": {"b"}, "b": {"a"}}, []string{"a", "b"}, "cycle"},
		{"3-cycle", map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"a"}}, []string{"a", "b", "c"}, "cycle"},
		{"unknown ref", map[string][]string{"a": {"ghost"}}, []string{"a"}, "unknown component"},
		{"self ref", map[string][]string{"a": {"a"}}, []string{"a"}, "references itself"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := componentWaves(tc.deps, tc.all)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.msg) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.msg)
			}
		})
	}
}
