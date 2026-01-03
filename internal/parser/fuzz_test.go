package parser

import (
	"testing"
)

// FuzzParseTransitions fuzzes the transition string parser.
// Input format: "EVENT->target:guard,EVENT2->target2" or "EVENT->target/action1;action2:guard"
func FuzzParseTransitions(f *testing.F) {
	// Seed corpus with valid inputs
	f.Add("START->running")
	f.Add("STOP->idle:canStop")
	f.Add("SUBMIT->processing/validate;notify")
	f.Add("APPROVE->approved/log:hasPermission")
	f.Add("A->b,C->d,E->f")
	f.Add("EVENT->target/action1;action2;action3:guard")
	f.Add("")
	f.Add("->")
	f.Add("EVENT->")
	f.Add("->target")
	f.Add("EVENT->target::")
	f.Add("A->b:g1,C->d:g2,E->f:g3")

	// Edge cases
	f.Add("X->y/a;b;c;d;e;f;g;h;i;j:guard")
	f.Add("VERY_LONG_EVENT_NAME->very_long_target_name/action:guard")
	f.Add("  SPACED  ->  target  ")
	f.Add("EVENT->target/")
	f.Add("EVENT->target:")

	f.Fuzz(func(t *testing.T, input string) {
		// The parser should not panic on any input
		_, err := parseTransitions(input)

		// If parsing succeeded, verify the result is consistent
		if err == nil && input != "" {
			result, err2 := parseTransitions(input)
			if err2 != nil {
				t.Errorf("inconsistent parsing: first succeeded, second failed: %v", err2)
			}
			if result == nil {
				return
			}

			// Verify each transition has required fields
			for i, trans := range result {
				if trans.Event == "" {
					t.Errorf("transition %d has empty event", i)
				}
				if trans.Target == "" {
					t.Errorf("transition %d has empty target", i)
				}
			}
		}
	})
}

// FuzzParseTransition fuzzes a single transition parser.
func FuzzParseTransition(f *testing.F) {
	// Seed corpus
	f.Add("START->running")
	f.Add("STOP->idle:canStop")
	f.Add("SUBMIT->processing/validate")
	f.Add("APPROVE->approved/log;notify:hasPermission")
	f.Add("")
	f.Add("->")
	f.Add("EVENT->")
	f.Add("->target")
	f.Add("nospace->target")
	f.Add("  spaces  ->  target  ")
	f.Add("EVENT->target/action1;action2;action3")
	f.Add("E->t:")
	f.Add("E->t/:")

	f.Fuzz(func(t *testing.T, input string) {
		// The parser should not panic on any input
		result, err := parseTransition(input)

		// If parsing succeeded, verify basic invariants
		if err == nil {
			if result.Event == "" {
				t.Error("successful parse should have non-empty event")
			}
			if result.Target == "" {
				t.Error("successful parse should have non-empty target")
			}
		}
	})
}

// FuzzToSnakeCase fuzzes the CamelCase to snake_case converter.
func FuzzToSnakeCase(f *testing.F) {
	// Seed corpus with various patterns
	f.Add("SimpleState")
	f.Add("HTTPServer")
	f.Add("XMLParser")
	f.Add("APIGateway")
	f.Add("myVariable")
	f.Add("alreadylowercase")
	f.Add("ALLCAPS")
	f.Add("MixedCASEString")
	f.Add("")
	f.Add("A")
	f.Add("AB")
	f.Add("ABC")
	f.Add("ABCdef")
	f.Add("abcDEF")
	f.Add("URLParser")
	f.Add("parseURL")
	f.Add("getHTTPResponse")
	f.Add("ID")
	f.Add("userID")
	f.Add("getUserID")
	f.Add("HTMLElement")

	f.Fuzz(func(t *testing.T, input string) {
		// The converter should not panic on any input
		result := toSnakeCase(input)

		// Verify basic properties
		if input == "" && result != "" {
			t.Error("empty input should produce empty output")
		}

		// Result should be lowercase (no uppercase letters)
		for _, r := range result {
			if r >= 'A' && r <= 'Z' {
				t.Errorf("result contains uppercase letter: %s", result)
				break
			}
		}

		// Result should only contain original chars (lowercased) and underscores
		for _, r := range result {
			isLower := r >= 'a' && r <= 'z'
			isDigit := r >= '0' && r <= '9'
			isUnderscore := r == '_'
			if !isLower && !isDigit && !isUnderscore {
				// Could be other unicode - that's fine, just verify it was in input
				found := false
				for _, inputR := range input {
					if inputR == r || (inputR >= 'A' && inputR <= 'Z' && inputR+32 == r) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("result contains character not in input: %c", r)
				}
			}
		}

		// Verify idempotence for already-lowercase strings
		if result == input {
			result2 := toSnakeCase(result)
			if result2 != result {
				t.Errorf("not idempotent for lowercase: %s -> %s -> %s", input, result, result2)
			}
		}
	})
}

// FuzzSplitTrim fuzzes the string splitting utility.
func FuzzSplitTrim(f *testing.F) {
	// Seed corpus
	f.Add("a,b,c", ",")
	f.Add("  a  ,  b  ,  c  ", ",")
	f.Add("action1;action2;action3", ";")
	f.Add("", ",")
	f.Add("single", ",")
	f.Add("  ", ",")
	f.Add("a,,b", ",")
	f.Add(",,,", ",")
	f.Add("no-separator-here", ",")

	f.Fuzz(func(t *testing.T, input, sep string) {
		// Skip if separator is empty (would cause infinite loop in strings.Split)
		if sep == "" {
			return
		}

		// The function should not panic
		result := splitTrim(input, sep)

		// Verify no result element is empty or whitespace-only
		for i, part := range result {
			if part == "" {
				t.Errorf("result %d is empty", i)
			}
			trimmed := part
			for _, r := range trimmed {
				if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
					continue
				}
				goto hasContent
			}
			t.Errorf("result %d is whitespace-only: %q", i, part)
		hasContent:
		}
	})
}
