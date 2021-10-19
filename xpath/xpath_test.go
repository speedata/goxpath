package xpath

import (
	"math"
	"testing"
)

func TestBooleanValue(t *testing.T) {
	testdata := []struct {
		input  Sequence
		output bool
	}{
		{Sequence{"hello"}, true},
		{Sequence{1.0}, true},
		{Sequence{math.NaN()}, false},
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
		result Sequence
	}{
		{`if ( false() ) then 'a' else 'b'`, Sequence{"b"}},
		{`if ( true() ) then 'a' else 'b'`, Sequence{"a"}},
		{`true()`, Sequence{true}},
		{`2`, Sequence{2.0}},
		{`1 to 3`, Sequence{1.0, 2.0, 3.0}},
		{`for $foo in 1 to 3 return $foo * 2`, Sequence{2.0, 4.0, 6.0}},
		{` +-+-+2`, Sequence{2.0}},
		{` +-+-+-+ 2`, Sequence{-2.0}},
		{`2 = 4`, Sequence{false}},
		{`2 = 2`, Sequence{true}},
		{`2 < 2`, Sequence{false}},
		{`2 < 3`, Sequence{true}},
		{`3.4 > 3.1`, Sequence{true}},
		{`3.4 != 3.1`, Sequence{true}},
		{`'abc' = 'abc'`, Sequence{true}},
		{`'aA' < 'aa'`, Sequence{true}},
		{`'aA' != 'aa'`, Sequence{true}},
		{`false() or true()`, Sequence{true}},
		{`false() or false()`, Sequence{false}},
		{`true() or false()`, Sequence{true}},
		{`true() and false()`, Sequence{false}},
		{`true() and true()`, Sequence{true}},
		{`4 < 2  or 5 < 7`, Sequence{true}},
		{`2 > 4 or 3 > 5 or 6 > 2`, Sequence{true}},
		{`4 < 2  or 7 < 5`, Sequence{false}},
		{`not( 3 < 6 )`, Sequence{false}},
		{`not( 6 < 3 )`, Sequence{true}},
		{`not( true() )`, Sequence{false}},
		{`concat('abc','def')`, Sequence{"abcdef"}},
		{`boolean(1)`, Sequence{true}},
		{`boolean(0)`, Sequence{false}},
		{`boolean(false())`, Sequence{false}},
		{`boolean(true())`, Sequence{true}},
		{`boolean('')`, Sequence{false}},
		{`boolean('false')`, Sequence{true}},
		// {`number('zzz')`, sequence{math.NaN()}},
		{`3 + 4 - 2`, Sequence{5.0}},
		{`$foo`, Sequence{"bar"}},
		{`$onedotfive + 2 `, Sequence{3.5}},
		{`$onedotfive * 2 `, Sequence{3.0}},
		{`7 mod 3 `, Sequence{1.0}},
		{`9 * 4 div 6 `, Sequence{6.0}},
		{`7 div 2 `, Sequence{3.5}},
		{`-3 div 2 `, Sequence{-1.5}},
		{`$one-two div $a`, Sequence{2.4}},
		{`$one-two idiv $a`, Sequence{2.0}},
		{`7 div 2 = 3.5 `, Sequence{true}},
		{`8 mod 2 = 0 `, Sequence{true}},
		{`(1,2) `, Sequence{1.0, 2.0}},
		{`(1,2) = (2,3) `, Sequence{true}},
		{`(1,2) != (2,3) `, Sequence{true}},
		{`(1,2) != (1,2) `, Sequence{true}},
		{`( 1,2,(),3 ) `, Sequence{1.0, 2.0, 3.0}},
		{`() `, Sequence{}},
		{`( () ) `, Sequence{}},
		{`3 , 3`, Sequence{3.0, 3.0}},
		{`(3 , 3)`, Sequence{3.0, 3.0}},
		{`(1,2)[true()]`, Sequence{1.0, 2.0}},
		{`(1,2)[false()]`, Sequence{}},
		{`( (),2 )[1]`, Sequence{2.0}},

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
		ctx := Context{
			vars: map[string]Sequence{
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
