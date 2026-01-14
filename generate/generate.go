// Package generate provides Go code generation from Statekit Native JSON definitions.
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

	"github.com/felixgeelhaar/statekit/viz"
)

// Generator generates Go code from Native JSON.
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

// Generate generates Go code from Native JSON.
func (g *Generator) Generate(r io.Reader) ([]byte, error) {
	var machine viz.VizMachine
	if err := json.NewDecoder(r).Decode(&machine); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	return g.GenerateMachine(&machine)
}

// GenerateMachine generates Go code from a parsed VizMachine.
func (g *Generator) GenerateMachine(machine *viz.VizMachine) ([]byte, error) {
	// Collect all actions and guards
	actions := make(map[string]bool)
	guards := make(map[string]bool)
	g.collectActionsAndGuards(machine, actions, guards)

	// Build template data

data := templateData{
		Package:     g.PackageName,
		TypeName:    g.TypeName,
		ContextType: g.ContextType,
		MachineID:   machine.ID,
		Initial:     machine.Initial,
		Actions:     sortedKeys(actions),
		Guards:      sortedKeys(guards),
		States:      g.buildStates(machine, machine.GetRootStates(), 1),
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

func (g *Generator) collectActionsAndGuards(vm *viz.VizMachine, actions, guards map[string]bool) {
	for _, state := range vm.States {
		for _, a := range state.Entry {
			actions[a] = true
		}
		for _, a := range state.Exit {
			actions[a] = true
		}
		for _, t := range state.Transitions {
			if t.Guard != "" {
				guards[t.Guard] = true
			}
			for _, a := range t.Actions {
				actions[a] = true
			}
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

func (g *Generator) buildStates(vm *viz.VizMachine, stateIDs []string, indent int) []stateData {
	var result []stateData

	// Sort state names for deterministic output
	sort.Strings(stateIDs)

	for _, id := range stateIDs {
		state := vm.States[id]
		if state == nil {
			continue
		}

		sd := stateData{
			ID:         state.ID,
			Initial:    state.Initial,
			IsFinal:    state.Type == viz.VizStateFinal,
			IsParallel: state.Type == viz.VizStateParallel,
			IsHistory:  state.Type == viz.VizStateHistory,
			Indent:     indent,
			EntryActions: state.Entry,
			ExitActions:  state.Exit,
		}

		if sd.IsHistory {
			sd.HistoryType = state.HistoryType
			if sd.HistoryType == "" {
				sd.HistoryType = "shallow"
			}
			sd.HistoryDefault = state.HistoryDefault
		}

		// Transitions
		for _, t := range state.Transitions {
			sd.Transitions = append(sd.Transitions, transitionData{
				Event:   t.Event,
				Target:  t.Target,
				Guard:   t.Guard,
				Actions: t.Actions,
			})
		}

		// Sort transitions for deterministic output
		sort.Slice(sd.Transitions, func(i, j int) bool {
			return sd.Transitions[i].Event < sd.Transitions[j].Event
		})

		// Nested states
		if len(state.Children) > 0 {
			sd.Children = g.buildStates(vm, state.Children, indent+1)
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