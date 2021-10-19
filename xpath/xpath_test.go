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
		{`2`, sequence{2.0}},
		{`1 to 3`, sequence{1.0, 2.0, 3.0}},
		{`for $foo in 1 to 3 return $foo * 2`, sequence{2.0, 4.0, 6.0}},
		{` +-+-+2`, sequence{2.0}},
		{` +-+-+-+ 2`, sequence{-2.0}},
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
		{`true() and false()`, sequence{false}},
		{`true() and true()`, sequence{true}},
		{`4 < 2  or 5 < 7`, sequence{true}},
		{`2 > 4 or 3 > 5 or 6 > 2`, sequence{true}},
		{`4 < 2  or 7 < 5`, sequence{false}},
		{`not( 3 < 6 )`, sequence{false}},
		{`not( 6 < 3 )`, sequence{true}},
		{`not( true() )`, sequence{false}},
		{`concat('abc','def')`, sequence{"abcdef"}},
		{`boolean(1)`, sequence{true}},
		{`boolean(0)`, sequence{false}},
		{`boolean(false())`, sequence{false}},
		{`boolean(true())`, sequence{true}},
		{`boolean('')`, sequence{false}},
		{`boolean('false')`, sequence{true}},
		// {`number('zzz')`, sequence{math.NaN()}},
		{`3 + 4 - 2`, sequence{5.0}},
		{`$foo`, sequence{"bar"}},
		{`$onedotfive + 2 `, sequence{3.5}},
		{`$onedotfive * 2 `, sequence{3.0}},
		{`7 mod 3 `, sequence{1.0}},
		{`9 * 4 div 6 `, sequence{6.0}},
		{`7 div 2 `, sequence{3.5}},
		{`-3 div 2 `, sequence{-1.5}},
		{`$one-two div $a`, sequence{2.4}},
		{`$one-two idiv $a`, sequence{2.0}},
		{`7 div 2 = 3.5 `, sequence{true}},
		{`8 mod 2 = 0 `, sequence{true}},
		{`(1,2) `, sequence{1.0, 2.0}},
		{`(1,2) = (2,3) `, sequence{true}},
		{`(1,2) != (2,3) `, sequence{true}},
		{`(1,2) != (1,2) `, sequence{true}},
		{`( 1,2,(),3 ) `, sequence{1.0, 2.0, 3.0}},
		{`() `, sequence{}},
		{`( () ) `, sequence{}},
		{`3 , 3`, sequence{3.0, 3.0}},
		{`(3 , 3)`, sequence{3.0, 3.0}},
		{`(1,2)[true()]`, sequence{1.0, 2.0}},
		{`(1,2)[false()]`, sequence{}},
		{`( (),2 )[1]`, sequence{2.0}},

		// assert_false(eval1(" boolean( (false()) )"))
		// assert_true(eval1("  boolean( (true()) )"))
		// assert_false(eval1(" boolean( () )"))

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
		ctx := context{
			vars: map[string]sequence{
				"foo":        {"bar"},
				"onedotfive": {1.5},
				"a":          {5.0},
				"two":        {2.0},
				"one":        {1.0},
				"one-two":    {12.0},
			},
		}
		seq, err := eval(&ctx)
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
