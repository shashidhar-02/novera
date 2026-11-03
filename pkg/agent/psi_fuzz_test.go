package agent

import (
	"strings"
	"testing"
)

// FuzzParsePSILines validates that ParsePSILines never panics or crashes on arbitrary byte input.
func FuzzParsePSILines(f *testing.F) {
	// Seed corpus with valid and edge-case samples
	f.Add("some avg10=2.45 avg60=1.12 avg300=0.50 total=1245000\nfull avg10=0.85 avg60=0.30 avg300=0.10 total=420000\n")
	f.Add("")
	f.Add("some\n")
	f.Add("some avg10=NaN avg60=Inf total=-1\n")
	f.Add("invalid line with no equals\n")
	f.Add(strings.Repeat("A", 10000))

	f.Fuzz(func(t *testing.T, input string) {
		// Must never panic
		_, _ = ParsePSILines(strings.NewReader(input))
	})
}
