package usp

import "testing"

func TestParse_Valid(t *testing.T) {
	tests := []struct {
		input          string
		wantOptOut     bool
		wantExplicit   Flag
	}{
		{"1YNN", false, FlagYes},
		{"1YYN", true, FlagYes},
		{"1NNN", false, FlagNo},
		{"1---", false, FlagNotApplicable},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			usp, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.input, err)
			}
			if usp.IsOptedOut() != tt.wantOptOut {
				t.Errorf("IsOptedOut: got %v, want %v", usp.IsOptedOut(), tt.wantOptOut)
			}
			if usp.ExplicitNotice != tt.wantExplicit {
				t.Errorf("ExplicitNotice: got %c, want %c", usp.ExplicitNotice, tt.wantExplicit)
			}
		})
	}
}

func TestParse_Invalid(t *testing.T) {
	tests := []string{
		"",
		"1YN",     // too short
		"1YNNN",   // too long
		"2YNN",    // version 2 not supported
		"1XNN",    // invalid flag X
	}
	for _, s := range tests {
		_, err := Parse(s)
		if err == nil {
			t.Errorf("Parse(%q) expected error", s)
		}
	}
}

func TestUSPrivacy_String(t *testing.T) {
	usp, _ := Parse("1YNN")
	if got := usp.String(); got != "1YNN" {
		t.Errorf("String() = %q, want %q", got, "1YNN")
	}
}
