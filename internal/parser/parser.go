// Package parser provides reflection-based parsing for struct-defined state machines.
package parser

import (
	"fmt"
	"reflect"
	"strings"
)

// StateSchemaType represents the type of a parsed state.
type StateSchemaType int

const (
	StateSchemaAtomic StateSchemaType = iota
	StateSchemaCompound
	StateSchemaFinal
)

// TransitionSchema represents a parsed transition definition.
type TransitionSchema struct {
	Event   string
	Target  string
	Guard   string
	Actions []string
}

// StateSchema represents a parsed state definition.
type StateSchema struct {
	Name        string
	Type        StateSchemaType
	Initial     string
	Entry       []string
	Exit        []string
	Transitions []TransitionSchema
	Children    []*StateSchema
}

// MachineSchema represents the complete parsed machine definition.
type MachineSchema struct {
	ID      string
	Initial string
	States  []*StateSchema
}

// Marker type names for detection.
const (
	MarkerMachineDefinition = "MachineDef"
	MarkerState             = "StateNode"
	MarkerCompoundState     = "CompoundNode"
	MarkerFinalState        = "FinalNode"
)

// ParseMachineStruct parses a struct type into a MachineSchema.
// The struct must have an embedded MachineDef marker type.
func ParseMachineStruct(t reflect.Type) (*MachineSchema, error) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", t.Kind())
	}

	schema := &MachineSchema{}

	// Find and parse the MachineDefinition marker
	found := false
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if isMarkerType(field.Type, MarkerMachineDefinition) {
			if err := parseMachineTag(field.Tag, schema); err != nil {
				return nil, fmt.Errorf("invalid machine tag: %w", err)
			}
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("struct must embed statekit.MachineDef")
	}

	// Parse state fields
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if isMarkerType(field.Type, MarkerMachineDefinition) {
			continue // Skip the machine definition marker
		}

		state, err := parseStateField(field)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}
		if state != nil {
			schema.States = append(schema.States, state)
		}
	}

	return schema, nil
}

// parseStateField parses a struct field into a StateSchema.
func parseStateField(field reflect.StructField) (*StateSchema, error) {
	fieldType := field.Type
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	// Check if this is a direct marker type or a struct containing one
	if fieldType.Kind() == reflect.Struct {
		// Check for embedded marker in the field's type
		markerType, hasMarker := findEmbeddedMarker(fieldType)
		if hasMarker {
			return parseStateStruct(field.Name, fieldType, markerType, field.Tag)
		}

		// Check if this field type itself is a marker
		if isMarkerType(fieldType, MarkerState) {
			return parseAtomicState(field.Name, field.Tag)
		}
		if isMarkerType(fieldType, MarkerCompoundState) {
			return parseCompoundState(field.Name, field.Tag)
		}
		if isMarkerType(fieldType, MarkerFinalState) {
			return parseFinalState(field.Name, field.Tag)
		}
	}

	return nil, nil // Not a state field
}

// findEmbeddedMarker finds an embedded marker type in a struct.
func findEmbeddedMarker(t reflect.Type) (string, bool) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.Anonymous {
			continue
		}
		for _, marker := range []string{MarkerState, MarkerCompoundState, MarkerFinalState} {
			if isMarkerType(field.Type, marker) {
				return marker, true
			}
		}
	}
	return "", false
}

// parseStateStruct parses a struct that contains an embedded marker.
func parseStateStruct(name string, t reflect.Type, markerType string, parentTag reflect.StructTag) (*StateSchema, error) {
	// Find the marker field to get its tag
	var markerTag reflect.StructTag
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous && isMarkerType(field.Type, markerType) {
			markerTag = field.Tag
			break
		}
	}

	// Use parent tag if marker has no tag
	tag := markerTag
	if tag == "" {
		tag = parentTag
	}

	switch markerType {
	case MarkerState:
		return parseAtomicState(name, tag)
	case MarkerCompoundState:
		state, err := parseCompoundState(name, tag)
		if err != nil {
			return nil, err
		}
		// Parse child states from non-marker fields
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.Anonymous {
				continue // Skip embedded marker
			}
			child, err := parseStateField(field)
			if err != nil {
				return nil, fmt.Errorf("child %s: %w", field.Name, err)
			}
			if child != nil {
				state.Children = append(state.Children, child)
			}
		}
		return state, nil
	case MarkerFinalState:
		return parseFinalState(name, tag)
	}

	return nil, fmt.Errorf("unknown marker type: %s", markerType)
}

// parseAtomicState parses an atomic state from a tag.
func parseAtomicState(name string, tag reflect.StructTag) (*StateSchema, error) {
	state := &StateSchema{
		Name: toSnakeCase(name),
		Type: StateSchemaAtomic,
	}

	if err := parseStateTag(tag, state); err != nil {
		return nil, err
	}

	return state, nil
}

// parseCompoundState parses a compound state from a tag.
func parseCompoundState(name string, tag reflect.StructTag) (*StateSchema, error) {
	state := &StateSchema{
		Name: toSnakeCase(name),
		Type: StateSchemaCompound,
	}

	if err := parseStateTag(tag, state); err != nil {
		return nil, err
	}

	return state, nil
}

// parseFinalState parses a final state from a tag.
func parseFinalState(name string, tag reflect.StructTag) (*StateSchema, error) {
	state := &StateSchema{
		Name: toSnakeCase(name),
		Type: StateSchemaFinal,
	}

	// Final states typically don't have transitions
	if err := parseStateTag(tag, state); err != nil {
		return nil, err
	}

	return state, nil
}

// parseMachineTag parses the machine definition tag.
// Format: `id:"machineId" initial:"stateName"`
func parseMachineTag(tag reflect.StructTag, schema *MachineSchema) error {
	schema.ID = tag.Get("id")
	schema.Initial = tag.Get("initial")

	if schema.ID == "" {
		return fmt.Errorf("missing required 'id' tag")
	}
	if schema.Initial == "" {
		return fmt.Errorf("missing required 'initial' tag")
	}

	return nil
}

// parseStateTag parses state-level tags.
// Format: `on:"EVENT->target:guard,EVENT2->target2" entry:"action1,action2" exit:"action3" initial:"child"`
func parseStateTag(tag reflect.StructTag, state *StateSchema) error {
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

// parseTransitions parses the transition string.
// Format: "EVENT->target:guard,EVENT2->target2" or "EVENT->target/action1,action2"
func parseTransitions(s string) ([]TransitionSchema, error) {
	var transitions []TransitionSchema

	parts := splitTrim(s, ",")
	for i, part := range parts {
		trans, err := parseTransition(part)
		if err != nil {
			return nil, fmt.Errorf("transition %d: %w", i+1, err)
		}
		transitions = append(transitions, trans)
	}

	return transitions, nil
}

// parseTransition parses a single transition.
// Format: "EVENT->target" or "EVENT->target:guard" or "EVENT->target/action1;action2:guard"
func parseTransition(s string) (TransitionSchema, error) {
	trans := TransitionSchema{}

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
	// Format: target:guard or target/actions:guard
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

// isMarkerType checks if a type matches a marker type name.
func isMarkerType(t reflect.Type, markerName string) bool {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name() == markerName
}

// toSnakeCase converts CamelCase to snake_case.
// Handles acronyms properly: HTTPServer -> http_server, APIGateway -> api_gateway.
func toSnakeCase(s string) string {
	if s == "" {
		return ""
	}

	runes := []rune(s)
	var result strings.Builder
	result.Grow(len(s) + 5) // Pre-allocate with some extra for underscores

	for i, r := range runes {
		isUpper := r >= 'A' && r <= 'Z'

		if i > 0 && isUpper {
			prevIsLower := runes[i-1] >= 'a' && runes[i-1] <= 'z'
			nextIsLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'

			// Insert underscore if:
			// - Previous char was lowercase (camelCase boundary), OR
			// - Next char is lowercase (end of acronym, e.g., the 'S' in HTTPServer)
			if prevIsLower || nextIsLower {
				result.WriteByte('_')
			}
		}

		if isUpper {
			result.WriteRune(r + 32) // Convert to lowercase
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
