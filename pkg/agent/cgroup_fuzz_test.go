package agent

import (
	"strings"
	"testing"
)

// FuzzParseCPUStat validates that ParseCPUStat never panics on arbitrary kernel string inputs.
func FuzzParseCPUStat(f *testing.F) {
	f.Add("usage_usec 15200000\nuser_usec 10100000\nsystem_usec 5100000\nnr_periods 1000\nnr_throttled 150\nthrottled_usec 750000\n")
	f.Add("")
	f.Add("corrupt line\n")
	f.Add("nr_periods 0\nnr_throttled 500\n")
	f.Add("usage_usec 999999999999999999999999999999999999\n")

	f.Fuzz(func(t *testing.T, input string) {
		_, _ = ParseCPUStat(strings.NewReader(input))
	})
}

