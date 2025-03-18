package regexanalysis

import (
	"math"
	"testing"
)

func TestAcceptedLength(t *testing.T) {
	testcases := []struct {
		name  string
		input string
		want  AcceptedLengths
	}{
		{
			name:  "Empty string",
			input: "",
			want:  AcceptedLengths{MinLength: 0, MaxLength: 0},
		},
		{
			name:  "Single character",
			input: "a",
			want:  AcceptedLengths{MinLength: 1, MaxLength: 1},
		},
		{
			name:  "Single character repeated",
			input: "a*",
			want:  AcceptedLengths{MinLength: 0, MaxLength: math.MaxUint},
		},
		{
			name:  "Single character repeated 2 to 3 times",
			input: "a{2,3}",
			want:  AcceptedLengths{MinLength: 2, MaxLength: 3},
		},
		{
			name:  "prefix and suffix",
			input: "foo.*bar",
			want:  AcceptedLengths{MinLength: 6, MaxLength: math.MaxUint64},
		},
		{
			name:  "prefix, suffix and infix",
			input: "foo.*bar.*baz",
			want:  AcceptedLengths{MinLength: 9, MaxLength: math.MaxUint64},
		},
		{
			name:  "bug#252",
			input: "RABA_([A-Za-z0-9+/]|%[0-9a-fA-F]{2}){32}",
			want:  AcceptedLengths{MinLength: 5 + 32, MaxLength: 5 + 3*32},
		},
		{
			name:  "bug#252 simplified",
			input: "(?:a|bb){64}",
			want:  AcceptedLengths{MinLength: 64, MaxLength: 128},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := AcceptedLength(tc.input)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			if got != tc.want {
				t.Errorf("Expected %v, got %v", tc.want, got)
			}
		})
	}
}
