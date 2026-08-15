package viz

import (
	"encoding/json"
	"testing"
)

// The exporter and the parser are two halves of one round-trip, and they
// disagreed about the shape of a transition group. export.transitionEntry
// builds a map per transition and collapseTransitionGroup then writes an
// object when the group has exactly one entry and an array otherwise — while
// the parser read exactly one of those shapes per field and silently produced
// nothing for the other.
//
// Nothing errored. `always` as an object failed the whole unmarshal (v1.13.0
// only, since it was v1.13.0 that started reading `always` at all); an `on`
// array or an actions list carrying a raise descriptor just yielded zero
// transitions and a state that looked genuinely terminal. A diagram missing an
// edge reads exactly like a machine that has no such edge.
func TestParseNativeJSONAcceptsEveryShapeTheExporterEmits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		json  string
		state string
		want  []VizTransition
		// always reports on VizState.Always instead of .Transitions.
		always bool
	}{
		{
			name:  "on: bare target string",
			json:  `{"id":"m","initial":"a","states":{"a":{"on":{"GO":"b"}},"b":{}}}`,
			state: "a",
			want:  []VizTransition{{Event: "GO", Target: "b"}},
		},
		{
			name:  "on: single object",
			json:  `{"id":"m","initial":"a","states":{"a":{"on":{"GO":{"target":"b","guard":"ok"}}},"b":{}}}`,
			state: "a",
			want:  []VizTransition{{Event: "GO", Target: "b", Guard: "ok"}},
		},
		{
			// collapseTransitionGroup emits an array once a second guarded
			// transition shares the event. This parsed to zero transitions.
			name:  "on: array of guarded alternatives",
			json:  `{"id":"m","initial":"a","states":{"a":{"on":{"GO":[{"target":"b","guard":"g1"},{"target":"c"}]}},"b":{},"c":{}}}`,
			state: "a",
			want: []VizTransition{
				{Event: "GO", Target: "b", Guard: "g1"},
				{Event: "GO", Target: "c"},
			},
		},
		{
			// transitionEntry widens actions from []string to []any to embed
			// raised events as xstate.raise descriptors. []string could not
			// hold that, so the unmarshal failed and the transition vanished.
			name:  "on: actions carrying an xstate.raise descriptor",
			json:  `{"id":"m","initial":"a","states":{"a":{"on":{"GO":{"target":"b","actions":["log",{"type":"xstate.raise","event":{"type":"DONE"}}]}}},"b":{}}}`,
			state: "a",
			want:  []VizTransition{{Event: "GO", Target: "b", Actions: []string{"log"}, Raise: []string{"DONE"}}},
		},
		{
			// An internal transition legitimately has no target. Dropping it
			// loses the actions it exists to run.
			name:  "on: internal transition with no target",
			json:  `{"id":"m","initial":"a","states":{"a":{"on":{"PING":{"internal":true,"actions":["count"]}}}}}`,
			state: "a",
			want:  []VizTransition{{Event: "PING", Actions: []string{"count"}, Internal: true}},
		},
		{
			// Native Always objects carry raise as a first-class field, not as
			// an xstate.raise action descriptor. Parsing only the XState shape
			// dropped the raise while keeping the transition.
			name:   "always: native raise field",
			json:   `{"id":"m","initial":"a","states":{"a":{"always":{"target":"b","raise":["OPENED"]}},"b":{}}}`,
			state:  "a",
			always: true,
			want:   []VizTransition{{Target: "b", Raise: []string{"OPENED"}}},
		},
		{
			// The regression lexora hit: one eventless transition collapses to
			// an object, and []VizTransition rejected it — failing the entire
			// ParseNativeJSON call, not just this field.
			name:   "always: single eventless transition collapses to an object",
			json:   `{"id":"m","initial":"a","states":{"a":{"always":{"target":"b"}},"b":{}}}`,
			state:  "a",
			always: true,
			want:   []VizTransition{{Target: "b"}},
		},
		{
			name:   "always: two or more stay an array",
			json:   `{"id":"m","initial":"a","states":{"a":{"always":[{"target":"b","guard":"g"},{"target":"c"}]},"b":{},"c":{}}}`,
			state:  "a",
			always: true,
			want: []VizTransition{
				{Target: "b", Guard: "g"},
				{Target: "c"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm, err := ParseNativeJSON([]byte(tc.json))
			if err != nil {
				t.Fatalf("ParseNativeJSON: %v", err)
			}
			st := vm.States[tc.state]
			if st == nil {
				t.Fatalf("state %q missing from %v", tc.state, keys(vm.States))
			}
			got := st.Transitions
			if tc.always {
				got = st.Always
			}
			assertTransitions(t, got, tc.want)
		})
	}
}

// A group whose entries are all unusable must not be reported as a successful
// parse of nothing — but a single bad entry must not discard its siblings
// either.
func TestParseNativeJSONSkipsOnlyTheUnusableEntry(t *testing.T) {
	t.Parallel()
	const in = `{"id":"m","initial":"a","states":{"a":{"on":{"GO":[{"target":"b"},42]}},"b":{}}}`
	vm, err := ParseNativeJSON([]byte(in))
	if err != nil {
		t.Fatalf("ParseNativeJSON: %v", err)
	}
	assertTransitions(t, vm.States["a"].Transitions, []VizTransition{{Event: "GO", Target: "b"}})
}

func assertTransitions(t *testing.T, got, want []VizTransition) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d transitions, want %d\ngot:  %s\nwant: %s", len(got), len(want), dump(got), dump(want))
	}
	// Map iteration over "on" makes sibling order unstable; compare as a set.
	remaining := append([]VizTransition(nil), want...)
	for _, g := range got {
		idx := -1
		for i, w := range remaining {
			if dump(g) == dump(w) {
				idx = i
				break
			}
		}
		if idx < 0 {
			t.Fatalf("unexpected transition %s\nwant one of: %s", dump(g), dump(remaining))
		}
		remaining = append(remaining[:idx], remaining[idx+1:]...)
	}
}

func dump(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func keys(m map[string]*VizState) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// `after` is the last field the exporter writes and the parser did not read.
//
// It is the same defect as `always` before v1.13.0 and it survived that fix:
// rawState simply had no After field, so every delayed transition parsed to
// nothing with no error. A machine whose only edge is a timeout rendered as
// two disconnected states, which reads as a modelling mistake rather than a
// parser gap. VizTransition has carried IsDelayed and DelayMs the whole time
// — nothing ever set them.
func TestParseNativeJSONReadsAfter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		json string
		want []VizTransition
	}{
		{
			// One transition per delay collapses to an object, exactly as
			// `always` and `on` do.
			name: "single delayed transition",
			json: `{"id":"m","initial":"a","states":{"a":{"after":{"5000":{"target":"b"}}},"b":{}}}`,
			want: []VizTransition{{Target: "b", IsDelayed: true, DelayMs: 5000}},
		},
		{
			name: "guarded alternatives on one delay",
			json: `{"id":"m","initial":"a","states":{"a":{"after":{"1000":[{"target":"b","guard":"g"},{"target":"c"}]}},"b":{},"c":{}}}`,
			want: []VizTransition{
				{Target: "b", Guard: "g", IsDelayed: true, DelayMs: 1000},
				{Target: "c", IsDelayed: true, DelayMs: 1000},
			},
		},
		{
			name: "several delays on one state",
			json: `{"id":"m","initial":"a","states":{"a":{"after":{"1000":{"target":"b"},"9000":{"target":"c"}}},"b":{},"c":{}}}`,
			want: []VizTransition{
				{Target: "b", IsDelayed: true, DelayMs: 1000},
				{Target: "c", IsDelayed: true, DelayMs: 9000},
			},
		},
		{
			// A non-numeric key is an XState delay *reference*. It cannot be
			// rendered as a duration, but dropping the edge loses the fact
			// that the state is not terminal.
			name: "named delay keeps the edge",
			json: `{"id":"m","initial":"a","states":{"a":{"after":{"TIMEOUT":{"target":"b"}}},"b":{}}}`,
			want: []VizTransition{{Event: "TIMEOUT", Target: "b", IsDelayed: true}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm, err := ParseNativeJSON([]byte(tc.json))
			if err != nil {
				t.Fatalf("ParseNativeJSON: %v", err)
			}
			assertTransitions(t, vm.States["a"].Transitions, tc.want)
		})
	}
}
