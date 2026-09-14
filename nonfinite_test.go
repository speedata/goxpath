package goxpath

import (
	"strings"
	"testing"
)

// TestNonFiniteToInt covers expressions whose evaluation used to convert a NaN,
// an infinity or an out-of-int-range float to an int. Go leaves that conversion
// up to the platform — arm64 saturates, amd64 yields MinInt — so these returned
// different results per architecture, and fn:subsequence even panicked on
// amd64. The QT3 suite covers them too, but it is an optional external checkout.
func TestNonFiniteToInt(t *testing.T) {
	testdata := []struct {
		input  string
		result Sequence
	}{
		// -INF + INF is NaN, and no position compares true against NaN.
		{`count(subsequence(1 to 10, xs:double("-INF"), xs:double("INF")))`, Sequence{0}},
		{`count(subsequence(1 to 10, 1, xs:double("-INF")))`, Sequence{0}},
		{`count(subsequence(1 to 10, xs:double("INF"), 1))`, Sequence{0}},
		{`count(subsequence(1 to 10, xs:double("NaN"), 3))`, Sequence{0}},
		{`count(subsequence(1 to 10, 2, 3))`, Sequence{3}},
		{`substring("12345", 0 div 0e0, 3)`, Sequence{""}},
		{`substring("12345", 1, 0 div 0e0)`, Sequence{""}},
		{`substring("12345", 2, 3)`, Sequence{"234"}},
		// The literal is MaxInt64, which does not survive a float64 round trip.
		{`xs:unsignedLong(9223372036854775807) or xs:unsignedLong(0)`, Sequence{true}},
		{`xs:unsignedLong(9223372036854775807) and xs:unsignedLong(0)`, Sequence{false}},
		// Comparing a float beyond the int range must not promote to an int.
		{`1e29 gt 0`, Sequence{true}},
		{`-1e29 lt 0`, Sequence{true}},
		{`10000000000000000000000000000.0 div 0.1 gt 0`, Sequence{true}},
		{`2.5 gt 2`, Sequence{true}},
		{`3 lt 3.5`, Sequence{true}},
	}

	for _, td := range testdata {
		p, err := NewParser(strings.NewReader(`<root/>`))
		if err != nil {
			t.Fatal(err)
		}
		seq, err := p.Evaluate(td.input)
		if err != nil {
			t.Errorf("Evaluate(%s) returned %v", td.input, err)
			continue
		}
		if got, want := len(seq), len(td.result); got != want {
			t.Errorf("len(seq) = %d, want %d. test: %s", got, want, td.input)
			continue
		}
		for i, itm := range seq {
			if !itemsEqual(itm, td.result[i]) {
				t.Errorf("seq[%d] = %#v, want %#v. test: %s", i, itm, td.result[i], td.input)
			}
		}
	}

	// Out-of-range casts are rejected rather than wrapping to a platform value.
	p, err := NewParser(strings.NewReader(`<root/>`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Evaluate(`xs:integer(1e29)`); err == nil {
		t.Error("xs:integer(1e29) should fail, got no error")
	}
}
