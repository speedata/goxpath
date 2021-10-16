package xpath

import (
	"math"
	"testing"
)

func TestBooleanValue(t *testing.T) {
	testdata := []struct {
		input  sequence
		output bool
	}{
		{sequence{"hello"}, true},
		{sequence{1.0}, true},
		{sequence{math.NaN()}, false},
	}
	for _, td := range testdata {
		bv, err := booleanValue(td.input)
		if err != nil {
			t.Error(err.Error())
		}
		if got, expected := bv, td.output; got != expected {
			t.Errorf("booleanValue(%v) = %t, want %t", td.input, got, expected)
		}
	}
}

func TestEval(t *testing.T) {
	testdata := []struct {
		input  string
		result sequence
	}{
		{`if ( false() ) then 'a' else 'b'`, sequence{"b"}},
		{`if ( true() ) then 'a' else 'b'`, sequence{"a"}},
		{`true()`, sequence{true}},
		{`2 = 4`, sequence{false}},
		{`2 = 2`, sequence{true}},
		{`2 < 2`, sequence{false}},
		{`2 < 3`, sequence{true}},
		{`3.4 > 3.1`, sequence{true}},
		{`3.4 != 3.1`, sequence{true}},
		{`'abc' = 'abc'`, sequence{true}},
		{`'aA' < 'aa'`, sequence{true}},
		{`'aA' != 'aa'`, sequence{true}},
		{`false() or true()`, sequence{true}},
		{`false() or false()`, sequence{false}},
		{`true() or false()`, sequence{true}},
	}
	for _, td := range testdata {
		tl, err := stringToTokenlist(td.input)
		if err != nil {
			t.Errorf(err.Error())
		}
		eval, err := parseXPath(tl)
		if err != nil {
			t.Errorf(err.Error())
		}
		seq, err := eval(context{})
		if err != nil {
			t.Errorf(err.Error())
		}
		if got, want := len(seq), len(td.result); got != want {
			t.Errorf("len(seq) = %d, want %d", got, want)
		}
		for i, itm := range seq {
			if itm != td.result[i] {
				t.Errorf("seq[%d] = %v, want %v. Test: %s", i, itm, td.result[i], td.input)
			}
		}
	}
}
