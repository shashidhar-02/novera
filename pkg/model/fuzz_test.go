package model

import (
	"encoding/json"
	"testing"
)

// FuzzWorkloadJSONValidation fuzzes JSON decoding and structural validation of Workloads.
func FuzzWorkloadJSONValidation(f *testing.F) {
	validJSON := `{"id":"task-1","namespace":"default","class":"LatencySensitive","cpu_request_millis":1000,"memory_request_bytes":1073741824,"slo":{"target_p99_ms":25.0}}`
	f.Add([]byte(validJSON))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"id":"","cpu_request_millis":-1}`))
	f.Add([]byte(`{"slo":{"target_p99_ms":-99999999}}`))
	f.Add([]byte(`{"class":"ExploitClass"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var w Workload
		if err := json.Unmarshal(data, &w); err == nil {
			// Validate must never panic on unmarshaled structs
			_ = w.Validate()
		}
	})
}

