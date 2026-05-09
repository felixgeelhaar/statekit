package viz

import (
	"testing"
)

// FuzzParseNativeJSON ensures the Native JSON parser does not panic
// on arbitrary input. Garbage in must produce a clean error, never
// a runtime panic.
func FuzzParseNativeJSON(f *testing.F) {
	// Valid seeds
	f.Add([]byte(`{"id":"m","initial":"a","states":{"a":{}}}`))
	f.Add([]byte(`{"id":"m","initial":"a","states":{"a":{"on":{"GO":"b"}},"b":{}}}`))
	f.Add([]byte(`{"id":"m","initial":"a","states":{"a":{"type":"final"}}}`))
	f.Add([]byte(`{"id":"m","initial":"a","states":{"a":{"states":{"x":{}},"initial":"x"}}}`))

	// Malformed seeds
	f.Add([]byte(`{`))
	f.Add([]byte(``))
	f.Add([]byte(`null`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`"string"`))
	f.Add([]byte(`{"id":"m","states":null}`))
	f.Add([]byte(`{"id":"m","states":{"a":{"on":null}}}`))
	f.Add([]byte(`{"id":"m","states":{"a":{"on":{"E":null}}}}`))
	f.Add([]byte(`{"id":"m","states":{"a":{"on":{"E":42}}}}`))
	f.Add([]byte(`{"id":"m","states":{"a":{"on":{"E":[]}}}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic, regardless of input
		_, _ = ParseNativeJSON(data)
	})
}
