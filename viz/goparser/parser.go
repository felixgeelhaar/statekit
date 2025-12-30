// Package goparser provides Go source code parsing to extract state machine definitions.
//
// This package analyzes Go source files to find struct types that define state machines
// using statekit's reflection DSL (MachineDef, StateNode, CompoundNode, FinalNode markers).
package goparser

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/felixgeelhaar/statekit/viz"
)

// Parser extracts VizMachine definitions from Go source code.
type Parser struct {
	// TypeFilter optionally filters which types to include.
	// If empty, all machine types are included.
	TypeFilter []string
}

// NewParser creates a new Go source parser.
func NewParser() *Parser {
	return &Parser{}
}

// WithTypeFilter sets the type filter for the parser.
func (p *Parser) WithTypeFilter(types ...string) *Parser {
	p.TypeFilter = types
	return p
}

// ParsePackage parses a Go package and extracts state machine definitions.
// The pattern can be a package path (e.g., "./examples/traffic_light")
// or a pattern (e.g., "./...").
func (p *Parser) ParsePackage(pattern string) ([]*viz.VizMachine, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedTypes |
			packages.NeedSyntax |
			packages.NeedTypesInfo,
		Tests: true, // Include test files
	}

	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}

	var machines []*viz.VizMachine

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			// Collect errors
			var errs []string
			for _, e := range pkg.Errors {
				errs = append(errs, e.Error())
			}
			return nil, fmt.Errorf("package errors in %s: %s", pkg.PkgPath, strings.Join(errs, "; "))
		}

		pkgMachines, err := p.parsePackage(pkg)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", pkg.PkgPath, err)
		}
		machines = append(machines, pkgMachines...)
	}

	return machines, nil
}

// parsePackage extracts machines from a single package.
func (p *Parser) parsePackage(pkg *packages.Package) ([]*viz.VizMachine, error) {
	var machines []*viz.VizMachine

	// Walk through all files in the package
	for _, file := range pkg.Syntax {
		fileMachines, err := p.parseFile(pkg, file)
		if err != nil {
			return nil, err
		}
		machines = append(machines, fileMachines...)
	}

	return machines, nil
}

// parseFile extracts machines from a single file.
func (p *Parser) parseFile(pkg *packages.Package, file *ast.File) ([]*viz.VizMachine, error) {
	var machines []*viz.VizMachine

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			// Check if this struct embeds MachineDef
			machineTag := findMachineDefTag(structType)
			if machineTag == "" {
				continue
			}

			// Apply type filter
			if len(p.TypeFilter) > 0 {
				found := false
				for _, t := range p.TypeFilter {
					if typeSpec.Name.Name == t {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			// Parse the machine definition
			machine, err := p.parseMachineStruct(pkg, typeSpec.Name.Name, structType, machineTag)
			if err != nil {
				return nil, fmt.Errorf("type %s: %w", typeSpec.Name.Name, err)
			}
			machines = append(machines, machine)
		}
	}

	return machines, nil
}

// findMachineDefTag finds the MachineDef embedding and returns its tag.
func findMachineDefTag(st *ast.StructType) string {
	for _, field := range st.Fields.List {
		// Check for embedded (anonymous) field
		if len(field.Names) != 0 {
			continue
		}

		// Check if the type is MachineDef
		if isMarkerType(field.Type, "MachineDef") {
			if field.Tag != nil {
				// Remove quotes from tag literal
				return strings.Trim(field.Tag.Value, "`")
			}
			return ""
		}
	}
	return ""
}

// parseMachineStruct parses a struct type into a VizMachine.
func (p *Parser) parseMachineStruct(pkg *packages.Package, typeName string, st *ast.StructType, machineTag string) (*viz.VizMachine, error) {
	// Parse machine-level tags
	tag := reflect.StructTag(machineTag)
	machineID := tag.Get("id")
	initial := tag.Get("initial")

	if machineID == "" {
		machineID = toSnakeCase(typeName)
	}

	machine := &viz.VizMachine{
		ID:      machineID,
		Initial: initial,
		States:  make(map[string]*viz.VizState),
	}

	// Parse state fields
	for _, field := range st.Fields.List {
		// Skip embedded MachineDef
		if len(field.Names) == 0 {
			continue
		}

		fieldName := field.Names[0].Name
		fieldTag := ""
		if field.Tag != nil {
			fieldTag = strings.Trim(field.Tag.Value, "`")
		}

		state, err := p.parseStateField(pkg, fieldName, field.Type, fieldTag)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", fieldName, err)
		}
		if state != nil {
			p.addState(machine, state, "")
		}
	}

	return machine, nil
}

// parseStateField parses a struct field into a VizState.
func (p *Parser) parseStateField(pkg *packages.Package, name string, typeExpr ast.Expr, tag string) (*viz.VizState, error) {
	stateID := toSnakeCase(name)

	// Check for direct marker types
	if isMarkerType(typeExpr, "StateNode") {
		return p.parseAtomicState(stateID, tag)
	}
	if isMarkerType(typeExpr, "FinalNode") {
		return p.parseFinalState(stateID, tag)
	}
	if isMarkerType(typeExpr, "CompoundNode") {
		// CompoundNode directly in field (rare, usually embedded in custom struct)
		return p.parseCompoundState(pkg, stateID, tag, nil)
	}

	// Check if field type is a custom struct with embedded marker
	ident, ok := typeExpr.(*ast.Ident)
	if !ok {
		return nil, nil // Not a state field
	}

	// Look up the type in the package
	obj := pkg.Types.Scope().Lookup(ident.Name)
	if obj == nil {
		return nil, nil // Type not found in this package
	}

	typeNamed, ok := obj.Type().(*types.Named)
	if !ok {
		return nil, nil
	}

	underlying, ok := typeNamed.Underlying().(*types.Struct)
	if !ok {
		return nil, nil
	}

	// Find what marker type is embedded
	markerType, markerTag, childFields := p.findEmbeddedMarker(underlying)
	if markerType == "" {
		return nil, nil // No marker embedded
	}

	// Use tag from the struct type's marker field, falling back to field-level tag
	effectiveTag := markerTag
	if effectiveTag == "" {
		effectiveTag = tag
	}

	switch markerType {
	case "StateNode":
		return p.parseAtomicState(stateID, effectiveTag)
	case "FinalNode":
		return p.parseFinalState(stateID, effectiveTag)
	case "CompoundNode":
		return p.parseCompoundState(pkg, stateID, effectiveTag, childFields)
	}

	return nil, nil
}

// findEmbeddedMarker finds the embedded marker type in a struct.
func (p *Parser) findEmbeddedMarker(st *types.Struct) (markerType string, tag string, otherFields []*types.Var) {
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)

		if field.Embedded() {
			typeName := getTypeName(field.Type())
			for _, marker := range []string{"StateNode", "CompoundNode", "FinalNode"} {
				if typeName == marker {
					return marker, st.Tag(i), otherFields
				}
			}
		} else {
			otherFields = append(otherFields, field)
		}
	}
	return "", "", nil
}

// parseAtomicState parses an atomic state from a tag.
func (p *Parser) parseAtomicState(stateID, tag string) (*viz.VizState, error) {
	state := &viz.VizState{
		ID:   stateID,
		Type: viz.VizStateAtomic,
	}

	if err := p.parseStateTag(tag, state); err != nil {
		return nil, err
	}

	return state, nil
}

// parseFinalState parses a final state from a tag.
func (p *Parser) parseFinalState(stateID, tag string) (*viz.VizState, error) {
	state := &viz.VizState{
		ID:   stateID,
		Type: viz.VizStateFinal,
	}

	if err := p.parseStateTag(tag, state); err != nil {
		return nil, err
	}

	return state, nil
}

// parseCompoundState parses a compound state.
//
//nolint:unparam // pkg reserved for future package-level analysis
func (p *Parser) parseCompoundState(pkg *packages.Package, stateID, tag string, childFields []*types.Var) (*viz.VizState, error) {
	state := &viz.VizState{
		ID:   stateID,
		Type: viz.VizStateCompound,
	}

	if err := p.parseStateTag(tag, state); err != nil {
		return nil, err
	}

	// Parse children from child fields
	// Note: This is simplified - full implementation would need AST access
	// to properly parse nested struct types
	for _, field := range childFields {
		childName := toSnakeCase(field.Name())
		childState := &viz.VizState{
			ID:     childName,
			Type:   viz.VizStateAtomic,
			Parent: stateID,
		}
		// We can't easily get the tag from types.Var, so children have minimal info
		state.Children = append(state.Children, childName)
		// Note: These children won't have full transition info without AST access
		_ = childState // Children would need separate handling
	}

	return state, nil
}

// parseStateTag parses state-level tags.
func (p *Parser) parseStateTag(tagStr string, state *viz.VizState) error {
	if tagStr == "" {
		return nil
	}

	tag := reflect.StructTag(tagStr)

	// Parse initial (for compound states)
	if initial := tag.Get("initial"); initial != "" {
		state.Initial = initial
	}

	// Parse entry actions
	if entry := tag.Get("entry"); entry != "" {
		state.Entry = splitTrim(entry, ",")
	}

	// Parse exit actions
	if exit := tag.Get("exit"); exit != "" {
		state.Exit = splitTrim(exit, ",")
	}

	// Parse transitions
	if on := tag.Get("on"); on != "" {
		transitions, err := parseTransitions(on)
		if err != nil {
			return fmt.Errorf("invalid 'on' tag: %w", err)
		}
		state.Transitions = transitions
	}

	return nil
}

// addState adds a state to the machine, setting parent relationships.
func (p *Parser) addState(m *viz.VizMachine, state *viz.VizState, parentID string) {
	state.Parent = parentID
	if parentID != "" {
		state.Depth = m.States[parentID].Depth + 1
	}
	m.States[state.ID] = state
}

// parseTransitions parses the transition string.
func parseTransitions(s string) ([]viz.VizTransition, error) {
	var transitions []viz.VizTransition

	parts := splitTrim(s, ",")
	for _, part := range parts {
		trans, err := parseTransition(part)
		if err != nil {
			return nil, err
		}
		transitions = append(transitions, trans)
	}

	return transitions, nil
}

// parseTransition parses a single transition.
// Format: "EVENT->target" or "EVENT->target:guard" or "EVENT->target/action1;action2:guard"
func parseTransition(s string) (viz.VizTransition, error) {
	trans := viz.VizTransition{}

	// Split on "->"
	arrowIdx := strings.Index(s, "->")
	if arrowIdx == -1 {
		return trans, fmt.Errorf("missing '->' in transition: %s", s)
	}

	trans.Event = strings.TrimSpace(s[:arrowIdx])
	rest := strings.TrimSpace(s[arrowIdx+2:])

	if trans.Event == "" {
		return trans, fmt.Errorf("empty event in transition: %s", s)
	}

	// Parse target, guard, and actions
	if colonIdx := strings.LastIndex(rest, ":"); colonIdx != -1 {
		trans.Guard = strings.TrimSpace(rest[colonIdx+1:])
		rest = rest[:colonIdx]
	}

	if slashIdx := strings.Index(rest, "/"); slashIdx != -1 {
		trans.Target = strings.TrimSpace(rest[:slashIdx])
		actionsStr := strings.TrimSpace(rest[slashIdx+1:])
		trans.Actions = splitTrim(actionsStr, ";")
	} else {
		trans.Target = strings.TrimSpace(rest)
	}

	if trans.Target == "" {
		return trans, fmt.Errorf("empty target in transition: %s", s)
	}

	return trans, nil
}

// isMarkerType checks if an expression references a marker type.
func isMarkerType(expr ast.Expr, markerName string) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name == markerName
	case *ast.SelectorExpr:
		// e.g., statekit.StateNode
		return e.Sel.Name == markerName
	}
	return false
}

// getTypeName extracts the type name from a types.Type.
func getTypeName(t types.Type) string {
	switch typ := t.(type) {
	case *types.Named:
		return typ.Obj().Name()
	case *types.Pointer:
		return getTypeName(typ.Elem())
	}
	return ""
}

// toSnakeCase converts CamelCase to snake_case.
func toSnakeCase(s string) string {
	if s == "" {
		return ""
	}

	runes := []rune(s)
	var result strings.Builder
	result.Grow(len(s) + 5)

	for i, r := range runes {
		isUpper := r >= 'A' && r <= 'Z'

		if i > 0 && isUpper {
			prevIsLower := runes[i-1] >= 'a' && runes[i-1] <= 'z'
			nextIsLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'

			if prevIsLower || nextIsLower {
				result.WriteByte('_')
			}
		}

		if isUpper {
			result.WriteRune(r + 32)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// splitTrim splits a string and trims whitespace from each part.
func splitTrim(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
