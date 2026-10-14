package main

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// SGR style classes found in an escape-sequence stream.
const (
	sgrColor16   = "color16"
	sgrColor256  = "color256"
	sgrTrueColor = "truecolor"
	sgrBold      = "bold"
	sgrFaint     = "faint"
	sgrOther     = "other" // any style not listed above, so new output is noticed
)

// sgrParams returns the parameters of every SGR sequence (CSI ... m) in s.
// It scans ESC '[' sequences, the only form lipgloss and colorprofile emit;
// the 8-bit C1 introducer (0x9B) is skipped because it is a valid UTF-8
// continuation byte. A sequence ends at the first ASCII final byte 0x40..0x7E
// (IndexFunc matches runes, but a match is always one ASCII byte, so the byte
// offset is valid) and only those ending in 'm' are returned. ';' separates
// parameters, an empty one means 0, a ':' subparameter group stays whole, and
// an unterminated sequence is an error.
func sgrParams(s string) ([][]string, error) {
	var seqs [][]string
	for {
		_, rest, found := strings.Cut(s, "\x1b[")
		if !found {
			return seqs, nil
		}
		s = rest
		end := strings.IndexFunc(s, func(r rune) bool { return r >= 0x40 && r <= 0x7e })
		if end < 0 {
			return nil, fmt.Errorf("unterminated escape sequence in %q", s)
		}
		params, final := s[:end], s[end]
		s = s[end+1:]
		if final != 'm' {
			continue
		}
		seqs = append(seqs, strings.Split(params, ";"))
	}
}

// sgrClassSet returns the sorted set of style classes used by the SGR
// sequences in s, and nil when there are none.
func sgrClassSet(s string) ([]string, error) {
	seqs, err := sgrParams(s)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, params := range seqs {
		for i := 0; i < len(params); i++ {
			class, skip, err := classifySGR(params[i:])
			if err != nil {
				return nil, err
			}
			if class != "" {
				set[class] = true
			}
			i += skip
		}
	}
	return slices.Sorted(maps.Keys(set)), nil
}

func sgrClasses(t *testing.T, s string) []string {
	t.Helper()
	got, err := sgrClassSet(s)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// checkSGR checks that output uses exactly the style classes in want. An
// empty want asserts no escape byte at all: sgrClassSet ignores non-SGR
// sequences, so the raw check also catches those.
func checkSGR(output string, want []string) error {
	if len(want) == 0 {
		if strings.Contains(output, "\x1b") {
			return fmt.Errorf("expected no escape sequences")
		}
		return nil
	}
	got, err := sgrClassSet(output)
	if err != nil {
		return err
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("SGR classes = %q, want %q", got, want)
	}
	return nil
}

func assertSGR(t *testing.T, output string, want []string) {
	t.Helper()
	if err := checkSGR(output, want); err != nil {
		t.Errorf("%v (output %q)", err, output)
	}
}

// sgrNum parses one SGR parameter; empty means 0.
func sgrNum(p string) (int, error) {
	if p == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return 0, fmt.Errorf("bad SGR parameter %q", p)
	}
	return n, nil
}

// isColorIntroducer reports whether n starts an extended color (foreground,
// background or underline).
func isColorIntroducer(n int) bool { return n == 38 || n == 48 || n == 58 }

// classifySGR classifies params[0] and reports how many following ';'
// parameters it consumed. A ':' group is self-contained and consumes none.
// Resets (0, 22, 39, 49, 59) have no class.
func classifySGR(params []string) (class string, skip int, err error) {
	num := func(p string) int {
		n, e := sgrNum(p)
		if e != nil && err == nil {
			err = e
		}
		return n
	}
	if sub := strings.Split(params[0], ":"); len(sub) > 1 {
		if isColorIntroducer(num(sub[0])) {
			switch num(sub[1]) {
			case 5:
				return sgrColor256, 0, err
			case 2:
				return sgrTrueColor, 0, err
			}
		}
		return sgrOther, 0, err
	}
	n := num(params[0])
	if err != nil {
		return "", 0, err
	}
	switch {
	case n == 0 || n == 22 || n == 39 || n == 49 || n == 59:
		return "", 0, nil
	case n == 1:
		return sgrBold, 0, nil
	case n == 2:
		return sgrFaint, 0, nil
	case isColorIntroducer(n) && len(params) > 1 && num(params[1]) == 5:
		return sgrColor256, 2, err
	case isColorIntroducer(n) && len(params) > 1 && num(params[1]) == 2:
		return sgrTrueColor, 4, err
	case (n >= 30 && n <= 37) || (n >= 40 && n <= 47) || (n >= 90 && n <= 97) || (n >= 100 && n <= 107):
		return sgrColor16, 0, nil
	}
	return sgrOther, 0, err
}

func TestSGRClasses(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"\x1b[38;2;1;2;3m", []string{sgrTrueColor}},
		{"\x1b[48;2;1;2;3m", []string{sgrTrueColor}},
		{"\x1b[38;5;1m", []string{sgrColor256}},
		{"\x1b[48;5;31m", []string{sgrColor256}},
		{"\x1b[38:5:196m", []string{sgrColor256}},
		{"\x1b[48:5:21m", []string{sgrColor256}},
		{"\x1b[58;5;9m", []string{sgrColor256}},
		{"\x1b[58:5:9m", []string{sgrColor256}},
		{"\x1b[58;2;1;2;3m", []string{sgrTrueColor}},
		{"\x1b[58:2::1:2:3m", []string{sgrTrueColor}},
		{"\x1b[58;2;1;2;3;1m", []string{sgrBold, sgrTrueColor}},
		{"\x1b[38:2::1:2:3m", []string{sgrTrueColor}},
		{"\x1b[38:2:0:1:2:3m", []string{sgrTrueColor}},
		{"\x1b[1;38;5;196;48;5;21;2m", []string{sgrBold, sgrColor256, sgrFaint}},
		{"\x1b[31;102m", []string{sgrColor16}},
		{"\x1b[;1m", []string{sgrBold}},
		{"\x1b[39;49m", nil},
		{"\x1b[0;22;59m", nil},
		{"\x1b[4m", []string{sgrOther}},
		{"\x1b[3m", []string{sgrOther}},
		{"\x1b[4:3m", []string{sgrOther}},
		{"\x1b[?25l", nil},
		{"\x1b[1@", nil},
		{"\x1b[1~", nil},
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

func TestSGRParams(t *testing.T) {
	tests := []struct {
		in      string
		want    [][]string
		wantErr bool
	}{
		{"a\x1b[38;5;1", nil, true},
		{"\x1b[", nil, true},
		{"\x1b[1m\x1b[2m", [][]string{{"1"}, {"2"}}, false},
		{"a\x1b[1mb\x1b[2mc", [][]string{{"1"}, {"2"}}, false},
		{"\x1b[1;2m", [][]string{{"1", "2"}}, false},
		{"\x1b[?25l\x1b[1m", [][]string{{"1"}}, false},
		{"plain", nil, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.in), func(t *testing.T) {
			got, err := sgrParams(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !slices.EqualFunc(got, tt.want, slices.Equal[[]string]) {
				t.Errorf("sgrParams = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckSGR(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    []string
		wantErr bool
	}{
		{"empty want plain text", "plain", nil, false},
		{"empty want SGR", "\x1b[1m", nil, true},
		{"empty want non-SGR", "\x1b[?25l", nil, true},
		{"wrong class", "\x1b[1;38;5;196m", []string{sgrBold, sgrColor16}, true},
		{"bold", "\x1b[1m", []string{sgrBold}, false},
		{"bold and 256", "\x1b[1;38;5;196m", []string{sgrBold, sgrColor256}, false},
		{"unterminated", "\x1b[1;38;5", []string{sgrBold}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkSGR(tt.output, tt.want); (err != nil) != tt.wantErr {
				t.Errorf("checkSGR err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
