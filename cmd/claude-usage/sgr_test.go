package main

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// styled is bold, faint text with a 256-color foreground (196) and
// background (21), so a leak of any style class shows up in the output.
const styled = "\x1b[1;2;38;5;196;48;5;21mhot\x1b[m"

// SGR style classes found in an escape-sequence stream.
const (
	sgrColor16  = "color16"
	sgrColor256 = "color256"
	sgrTrue     = "truecolor"
	sgrBold     = "bold"
	sgrFaint    = "faint"
	sgrOther    = "other" // any style not listed above, so new output is noticed
)

// sgrParams returns the parameters of every SGR sequence (CSI ... m) in s.
// Only the ESC '[' introducer is recognized, the only form lipgloss and
// colorprofile emit. The 8-bit C1 form (0x9B) is not scanned: that byte is a
// valid UTF-8 continuation byte and would misread real text.
// Any byte in 0x40..0x7E ends a CSI sequence; only 'm' ones are returned.
// Parameters split on ';' only. An empty parameter is 0, and a parameter
// holding ':' subfields (38:5:n) stays one group.
func sgrParams(t *testing.T, s string) [][]string {
	t.Helper()
	var seqs [][]string
	for {
		start := strings.Index(s, "\x1b[")
		if start < 0 {
			return seqs
		}
		s = s[start+2:]
		end := strings.IndexFunc(s, func(r rune) bool { return r >= 0x40 && r <= 0x7e })
		if end < 0 {
			t.Fatalf("unterminated escape sequence in %q", s)
		}
		params, final := s[:end], s[end]
		s = s[end+1:]
		if final != 'm' {
			continue
		}
		seqs = append(seqs, strings.Split(params, ";"))
	}
}

// sgrClasses returns the sorted set of style classes used by the SGR
// sequences in s, and nil when there are none.
func sgrClasses(t *testing.T, s string) []string {
	t.Helper()
	set := map[string]bool{}
	for _, params := range sgrParams(t, s) {
		for i := 0; i < len(params); i++ {
			class, skip := classifySGR(t, params[i:])
			if class != "" {
				set[class] = true
			}
			i += skip
		}
	}
	return slices.Sorted(maps.Keys(set))
}

// sgrNum parses one SGR parameter; empty means 0.
func sgrNum(t *testing.T, p string) int {
	t.Helper()
	if p == "" {
		return 0
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		t.Fatalf("bad SGR parameter %q", p)
	}
	return n
}

// isColorIntroducer reports whether n starts an extended color (foreground,
// background or underline).
func isColorIntroducer(n int) bool { return n == 38 || n == 48 || n == 58 }

// classifySGR classifies params[0] and reports how many following ';'
// parameters it consumed. A ':' group is self-contained and consumes none.
// Resets (0, 22, 39, 49, 59) have no class.
func classifySGR(t *testing.T, params []string) (class string, skip int) {
	t.Helper()
	if sub := strings.Split(params[0], ":"); len(sub) > 1 {
		if isColorIntroducer(sgrNum(t, sub[0])) {
			switch sgrNum(t, sub[1]) {
			case 5:
				return sgrColor256, 0
			case 2:
				return sgrTrue, 0
			}
		}
		return sgrOther, 0
	}
	n := sgrNum(t, params[0])
	switch {
	case n == 0 || n == 22 || n == 39 || n == 49 || n == 59:
		return "", 0
	case n == 1:
		return sgrBold, 0
	case n == 2:
		return sgrFaint, 0
	case isColorIntroducer(n) && len(params) > 1 && sgrNum(t, params[1]) == 5:
		return sgrColor256, 2
	case isColorIntroducer(n) && len(params) > 1 && sgrNum(t, params[1]) == 2:
		return sgrTrue, 4
	case (n >= 30 && n <= 37) || (n >= 40 && n <= 47) || (n >= 90 && n <= 97) || (n >= 100 && n <= 107):
		return sgrColor16, 0
	}
	return sgrOther, 0
}

func TestSGRClasses(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"\x1b[38;2;1;2;3m", []string{sgrTrue}},
		{"\x1b[48;2;1;2;3m", []string{sgrTrue}},
		{"\x1b[38;5;1m", []string{sgrColor256}},
		{"\x1b[48;5;31m", []string{sgrColor256}},
		{"\x1b[38:5:196m", []string{sgrColor256}},
		{"\x1b[48:5:21m", []string{sgrColor256}},
		{"\x1b[58;5;9m", []string{sgrColor256}},
		{"\x1b[58:5:9m", []string{sgrColor256}},
		{"\x1b[58;2;1;2;3m", []string{sgrTrue}},
		{"\x1b[58:2::1:2:3m", []string{sgrTrue}},
		{"\x1b[58;2;1;2;3;1m", []string{sgrBold, sgrTrue}},
		{"\x1b[38:2::1:2:3m", []string{sgrTrue}},
		{"\x1b[38:2:0:1:2:3m", []string{sgrTrue}},
		{"\x1b[1;38;5;196;48;5;21;2m", []string{sgrBold, sgrColor256, sgrFaint}},
		{"\x1b[31;102m", []string{sgrColor16}},
		{"\x1b[;1m", []string{sgrBold}},
		{"\x1b[39;49m", nil},
		{"\x1b[0;22;59m", nil},
		{"\x1b[4m", []string{sgrOther}},
		{"\x1b[3m", []string{sgrOther}},
		{"\x1b[4:3m", []string{sgrOther}},
		{"\x1b[?25l", nil},
		{"\x1b[m", nil},
		{"\x1b]0;title\x07", nil},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.in), func(t *testing.T) {
			if got := sgrClasses(t, tt.in); !slices.Equal(got, tt.want) {
				t.Errorf("sgrClasses = %q, want %q", got, tt.want)
			}
		})
	}
}
