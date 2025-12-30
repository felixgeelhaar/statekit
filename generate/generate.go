// Package generate provides Go code generation from XState JSON definitions.
package generate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"io"
	"sort"
	"strings"
	"text/template"
	"unicode"
)

// Generator generates Go code from XState JSON.
type Generator struct {
	PackageName string
	TypeName    string
	ContextType string
}

// NewGenerator creates a new code generator.
func NewGenerator(packageName, typeName, contextType string) *Generator {
	if packageName == "" {
		packageName = "main"
	}
	if typeName == "" {
		typeName = "Machine"
	}
	if contextType == "" {
		contextType = "struct{}"
	}
	return &Generator{
		PackageName: packageName,
		TypeName:    typeName,
		ContextType: contextType,
	}
}

// XStateMachine represents an XState machine definition.
type XStateMachine struct {
	ID      string                `json:"id"`
	Initial string                `json:"initial"`
	Context json.RawMessage       `json:"context,omitempty"`
	States  map[string]XState     `json:"states"`
	On      map[string]XTransSpec `json:"on,omitempty"`
}

// XState represents an XState state definition.
type XState struct {
	Initial string                `json:"initial,omitempty"`
	Type    string                `json:"type,omitempty"`    // "final", "parallel", "history"
	History string                `json:"history,omitempty"` // "shallow", "deep"
	Target  string                `json:"target,omitempty"`  // default for history
	States  map[string]XState     `json:"states,omitempty"`
	On      map[string]XTransSpec `json:"on,omitempty"`
	Entry   XActionSpec           `json:"entry,omitempty"`
	Exit    XActionSpec           `json:"exit,omitempty"`
	After   map[string]XTransSpec `json:"after,omitempty"`
	Invoke  json.RawMessage       `json:"invoke,omitempty"`
}

// XTransSpec can be a string, object, or array of transitions.
type XTransSpec struct {
	raw json.RawMessage
}

// UnmarshalJSON implements json.Unmarshaler.
func (ts *XTransSpec) UnmarshalJSON(data []byte) error {
	ts.raw = make(json.RawMessage, len(data))
	copy(ts.raw, data)
	return nil
}

// XTransition represents a single transition.
type XTransition struct {
	Target  string      `json:"target,omitempty"`
	Guard   string      `json:"guard,omitempty"`
	Cond    string      `json:"cond,omitempty"` // alias for guard
	Actions XActionSpec `json:"actions,omitempty"`
}

// XActionSpec can be a string or array of action names.
type XActionSpec struct {
	raw json.RawMessage
}

// UnmarshalJSON implements json.Unmarshaler.
func (as *XActionSpec) UnmarshalJSON(data []byte) error {
	as.raw = make(json.RawMessage, len(data))
	copy(as.raw, data)
	return nil
}

// ParseTransitions parses the transition spec into transitions.
func (ts XTransSpec) ParseTransitions() ([]XTransition, error) {
	if len(ts.raw) == 0 {
		return nil, nil
	}

	// Try as string first
	var target string
	if err := json.Unmarshal(ts.raw, &target); err == nil {
		return []XTransition{{Target: target}}, nil
	}

	// Try as single object
	var single XTransition
	if err := json.Unmarshal(ts.raw, &single); err == nil {
		return []XTransition{single}, nil
	}

	// Try as array
	var arr []XTransition
	if err := json.Unmarshal(ts.raw, &arr); err == nil {
		return arr, nil
	}

	return nil, fmt.Errorf("invalid transition spec: %s", string(ts.raw))
}

// ParseActions parses action spec into action names.
func (as XActionSpec) ParseActions() ([]string, error) {
	if len(as.raw) == 0 {
		return nil, nil
	}

	// Try as string
	var single string
	if err := json.Unmarshal(as.raw, &single); err == nil {
		if single == "" {
			return nil, nil
		}
		return []string{single}, nil
	}

	// Try as array
	var arr []string
	if err := json.Unmarshal(as.raw, &arr); err == nil {
		return arr, nil
	}

	return nil, fmt.Errorf("invalid action spec: %s", string(as.raw))
}

// Generate generates Go code from XState JSON.
func (g *Generator) Generate(r io.Reader) ([]byte, error) {
	var machine XStateMachine
	if err := json.NewDecoder(r).Decode(&machine); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	return g.GenerateMachine(&machine)
}

// GenerateMachine generates Go code from a parsed XState machine.
func (g *Generator) GenerateMachine(machine *XStateMachine) ([]byte, error) {
	// Collect all actions and guards
	actions := make(map[string]bool)
	guards := make(map[string]bool)
	g.collectActionsAndGuards(machine.States, actions, guards)

	// Build template data
	data := templateData{
		Package:     g.PackageName,
		TypeName:    g.TypeName,
		ContextType: g.ContextType,
		MachineID:   machine.ID,
		Initial:     machine.Initial,
		Actions:     sortedKeys(actions),
		Guards:      sortedKeys(guards),
		States:      g.buildStates(machine.States, 1),
	}

	// Execute template
	var buf bytes.Buffer
	if err := codeTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	// Format Go code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Return unformatted if formatting fails (for debugging)
		return buf.Bytes(), fmt.Errorf("format code: %w", err)
	}

	return formatted, nil
}

func (g *Generator) collectActionsAndGuards(states map[string]XState, actions, guards map[string]bool) {
	for _, state := range states {
		// Entry actions
		if entryActions, _ := state.Entry.ParseActions(); len(entryActions) > 0 {
			for _, a := range entryActions {
				actions[a] = true
			}
		}

		// Exit actions
		if exitActions, _ := state.Exit.ParseActions(); len(exitActions) > 0 {
			for _, a := range exitActions {
				actions[a] = true
			}
		}

		// Transitions
		for _, transSpec := range state.On {
			transitions, _ := transSpec.ParseTransitions()
			for _, t := range transitions {
				if t.Guard != "" {
					guards[t.Guard] = true
				}
				if t.Cond != "" {
					guards[t.Cond] = true
				}
				if transActions, _ := t.Actions.ParseActions(); len(transActions) > 0 {
					for _, a := range transActions {
						actions[a] = true
					}
				}
			}
		}

		// Nested states
		if len(state.States) > 0 {
			g.collectActionsAndGuards(state.States, actions, guards)
		}
	}
}

type templateData struct {
	Package     string
	TypeName    string
	ContextType string
	MachineID   string
	Initial     string
	Actions     []string
	Guards      []string
	States      []stateData
}

type stateData struct {
	ID             string
	Initial        string
	IsFinal        bool
	IsParallel     bool
	IsHistory      bool
	HistoryType    string // "shallow" or "deep"
	HistoryDefault string
	EntryActions   []string
	ExitActions    []string
	Transitions    []transitionData
	Children       []stateData
	Indent         int
}

type transitionData struct {
	Event   string
	Target  string
	Guard   string
	Actions []string
}

func (g *Generator) buildStates(states map[string]XState, indent int) []stateData {
	var result []stateData

	// Sort state names for deterministic output
	names := make([]string, 0, len(states))
	for name := range states {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		state := states[name]
		sd := stateData{
			ID:         name,
			Initial:    state.Initial,
			IsFinal:    state.Type == "final",
			IsParallel: state.Type == "parallel",
			IsHistory:  state.Type == "history",
			Indent:     indent,
		}

		if sd.IsHistory {
			sd.HistoryType = state.History
			if sd.HistoryType == "" {
				sd.HistoryType = "shallow"
			}
			sd.HistoryDefault = state.Target
		}

		// Entry actions
		if actions, _ := state.Entry.ParseActions(); len(actions) > 0 {
			sd.EntryActions = actions
		}

		// Exit actions
		if actions, _ := state.Exit.ParseActions(); len(actions) > 0 {
			sd.ExitActions = actions
		}

		// Transitions
		for event, transSpec := range state.On {
			transitions, _ := transSpec.ParseTransitions()
			for _, t := range transitions {
				guard := t.Guard
				if guard == "" {
					guard = t.Cond
				}
				transActions, _ := t.Actions.ParseActions()
				sd.Transitions = append(sd.Transitions, transitionData{
					Event:   event,
					Target:  t.Target,
					Guard:   guard,
					Actions: transActions,
				})
			}
		}

		// Sort transitions for deterministic output
		sort.Slice(sd.Transitions, func(i, j int) bool {
			return sd.Transitions[i].Event < sd.Transitions[j].Event
		})

		// Nested states
		if len(state.States) > 0 {
			sd.Children = g.buildStates(state.States, indent+1)
		}

		result = append(result, sd)
	}

	return result
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// toGoIdentifier converts a string to a valid Go identifier.
func toGoIdentifier(s string) string {
	var result strings.Builder
	capitalize := true

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if capitalize {
				result.WriteRune(unicode.ToUpper(r))
				capitalize = false
			} else {
				result.WriteRune(r)
			}
		} else {
			capitalize = true
		}
	}

	return result.String()
}

var templateFuncs = template.FuncMap{
	"goIdent": toGoIdentifier,
	"indent": func(n int) string {
		return strings.Repeat("\t", n)
	},
}

var codeTemplate = template.Must(template.New("code").Funcs(templateFuncs).Parse(`// Code generated by statekit generate. DO NOT EDIT.
package {{.Package}}

import (
	"github.com/felixgeelhaar/statekit"
)

// {{.TypeName}}Context is the context type for the state machine.
// TODO: Replace with your actual context type.
type {{.TypeName}}Context = {{.ContextType}}

{{if .Actions}}
// Action stubs - implement these functions.
{{range .Actions}}
func action{{. | goIdent}}(ctx *{{$.TypeName}}Context, e statekit.Event) {
	// TODO: Implement {{.}} action
}
{{end}}
{{end}}

{{if .Guards}}
// Guard stubs - implement these functions.
{{range .Guards}}
func guard{{. | goIdent}}(ctx {{$.TypeName}}Context, e statekit.Event) bool {
	// TODO: Implement {{.}} guard
	return true
}
{{end}}
{{end}}

// Build{{.TypeName}} creates the {{.MachineID}} state machine.
func Build{{.TypeName}}() (*statekit.MachineConfig[{{.TypeName}}Context], error) {
	return statekit.NewMachine[{{.TypeName}}Context]("{{.MachineID}}").
		WithInitial("{{.Initial}}").
{{- range .Actions}}
		WithAction("{{.}}", action{{. | goIdent}}).
{{- end}}
{{- range .Guards}}
		WithGuard("{{.}}", guard{{. | goIdent}}).
{{- end}}
{{- template "states" .States}}
		Build()
}

{{define "states"}}
{{- range .}}
{{- $indent := .Indent}}
{{indent $indent}}State("{{.ID}}").
{{- if .IsFinal}}
{{indent $indent}}	Final().
{{- end}}
{{- if .IsParallel}}
{{indent $indent}}	Parallel().
{{- end}}
{{- if .Initial}}
{{indent $indent}}	WithInitial("{{.Initial}}").
{{- end}}
{{- range .EntryActions}}
{{indent $indent}}	OnEntry("{{.}}").
{{- end}}
{{- range .ExitActions}}
{{indent $indent}}	OnExit("{{.}}").
{{- end}}
{{- range .Transitions}}
{{indent $indent}}	On("{{.Event}}").Target("{{.Target}}"){{if .Guard}}.Guard("{{.Guard}}"){{end}}{{range .Actions}}.Do("{{.}}"){{end}}.
{{- end}}
{{- if .Children}}
{{- template "states" .Children}}
{{- end}}
{{indent $indent}}Done().
{{- end}}
{{- end}}
`))
