package goxpath

import (
	"math"
	"strings"
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
		bv, err := BooleanValue(td.input)
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
		{`substring("öäü", 2) `, Sequence{"äü"}},
		{`substring("Goldfarb", 5) `, Sequence{"farb"}},
		{`substring("Goldfarb", 5, 3) `, Sequence{"far"}},
		{`contains((), "a")`, Sequence{false}},
		{`contains("", "")`, Sequence{true}},
		{`contains("Shakespeare", "")`, Sequence{true}},
		{`contains("", "a")`, Sequence{false}},
		{`contains("Shakespeare", "spear")`, Sequence{true}},
		{`codepoints-to-string( reverse(  string-to-codepoints('Hellö')  ) ) `, Sequence{"ölleH"}},
		{`reverse( ( 1,2,3  ) ) `, Sequence{3.0, 2.0, 1.0}},
		{`string-to-codepoints( "hellö" ) `, Sequence{104, 101, 108, 108, 246}},
		{`codepoints-to-string( (65,33*2,67) )`, Sequence{"ABC"}},
		{`matches("abracadabra", "bra")`, Sequence{true}},
		{`matches("banana", "^(.a)+$")`, Sequence{true}},
		{`matches("", "a*")`, Sequence{true}},
		{`matches("23 May 2008", "^[0-9]+\s[A-Z][a-z]+\s[0-9]+$")`, Sequence{true}},
		{`replace("facetiously", "[aeiouy]", "[$0]")`, Sequence{"f[a]c[e]t[i][o][u]sl[y]"}},
		{`replace("banana", "(an)+?", "**")`, Sequence{"b****a"}},
		{`replace("banana", "(an)+", "**")`, Sequence{"b**a"}},
		{`replace("banana", "(ana|na)", "[$1]")`, Sequence{"b[ana][na]"}},
		{`replace("banana", "a", "o")`, Sequence{"bonono"}},
		{`tokenize("12, 16, 2", ",\s*")"`, Sequence{"12", "16", "2"}},
		{`tokenize("abc[NL]def[XY]", "\[.*?\]")"`, Sequence{"abc", "def", ""}},
		{`tokenize("Go home, Jack!","\W+")`, Sequence{"Go", "home", "Jack", ""}},
		{`count(/root/other | /root/other)`, Sequence{2}},
		{`/root/@zzz instance of attribute()+`, Sequence{false}},
		{`/root/@foo instance of attribute()+`, Sequence{true}},
		{`/root/sub instance of element()?`, Sequence{false}},
		{`/root/sub[1] instance of element()?`, Sequence{true}},
		{`/root/sub[1] instance of element()`, Sequence{true}},
		{`/root/sub instance of element()`, Sequence{false}},
		{`/root/sub instance of element()+`, Sequence{true}},
		{`/root/sub instance of element()*`, Sequence{true}},
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
		{`concat('abc','def')`, Sequence{"abcdef"}},
		{`string(number('zzz')) = 'NaN'`, Sequence{true}},
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
		{`10 idiv 3`, Sequence{3.0}},
		{`3 idiv -2`, Sequence{-1.0}},
		{`-3 idiv 2`, Sequence{-1.0}},
		{`-3 idiv -2`, Sequence{1.0}},
		{`9.0 idiv 3`, Sequence{3.0}},
		{`-3.5 idiv 3`, Sequence{-1.0}},
		{`-3.5 idiv 3`, Sequence{-1.0}},
		{`3.0 idiv 4`, Sequence{0.0}},
		{`7 div 2 = 3.5 `, Sequence{true}},
		{`8 mod 2 = 0 `, Sequence{true}},
		{`(1,2) `, Sequence{1.0, 2.0}},
		{`(1,2) = (2,3) `, Sequence{true}},
		{`(1,2) != (2,3) `, Sequence{true}},
		{`(1,2) != (1,2) `, Sequence{true}},
		{`( 1,2,(),3 ) `, Sequence{1.0, 2.0, 3.0}},
		{`/root/sub/position() `, Sequence{1, 2, 3}},
		{`/root/sub/last() `, Sequence{3, 3, 3}},
		{`/root/sub[@foo='bar']/last()`, Sequence{2, 2}},
		{`(/root/sub[@foo='bar']/last())[1]`, Sequence{2}},
		{`( /root/@doesnotexist , 'str' )[1] = 'str'`, Sequence{true}},
		{`( 'str', /root/@doesnotexist )[1] = 'str'`, Sequence{true}},
		{`() `, Sequence{}},
		{`( () ) `, Sequence{}},
		{`3 , 3`, Sequence{3.0, 3.0}},
		{`(3 , 3)`, Sequence{3.0, 3.0}},
		{`(1,2)[true()]`, Sequence{1.0, 2.0}},
		{`(1,2)[false()]`, Sequence{}},
		{`( (),2 )[1]`, Sequence{2.0}},
		{`( (),2 )[position() = 1]`, Sequence{2.0}},
		{`for $i in /root/other/@*[1] return string($i) `, Sequence{"barbaz", "other2"}},
		{`string(/root/sub[position() mod 2 = 0]/@foo)`, Sequence{"bar"}},
		{`string(/root/sub[last()]/@self)`, Sequence{"sub3"}},
		{`abs(2.0)`, Sequence{2.0}},
		{`abs(- 2)`, Sequence{2.0}},
		{`abs( -3.7 )`, Sequence{3.7}},
		{`abs( -1.0e-7 )`, Sequence{1.0e-7}},
		{`string(abs( 'NaN' ))`, Sequence{"NaN"}},
		{`boolean(1)`, Sequence{true}},
		{`boolean(0)`, Sequence{false}},
		{`boolean(false())`, Sequence{false}},
		{`boolean( (true()) )`, Sequence{true}},
		{`boolean( ((true())) )`, Sequence{true}},
		{`boolean(true())`, Sequence{true}},
		{`boolean('')`, Sequence{false}},
		{`boolean( () )`, Sequence{false}},
		{`boolean( (()) )`, Sequence{false}},
		{`boolean('false')`, Sequence{true}},
		{`ceiling(1.0)`, Sequence{1.0}},
		{`ceiling(1.6)`, Sequence{2.0}},
		{`ceiling(17 div 3)`, Sequence{6.0}},
		{`ceiling(-3)`, Sequence{-3.0}},
		{`ceiling(-8.2e0 )`, Sequence{-8.0e0}},
		{`ceiling( -0.5e0 )`, Sequence{-0.0}},
		{`string(ceiling('xxx' ))`, Sequence{"NaN"}},
		{`count(/root/sub)`, Sequence{3}},
		{`count(/root/a/*)`, Sequence{4}},
		{`count(/root/sub/subsub)`, Sequence{1}},
		{`count(/root/other)`, Sequence{2}},
		{`count(/root/a/sub[position() = 1])`, Sequence{2}},
		{`(count(/root/a/sub)[position() = 1])`, Sequence{4}},
		{`count( (/root/a/sub)[position() = 2]) `, Sequence{1}},
		{`count(/root/a/sub[1])`, Sequence{2}},
		{`(count(/root/a/sub)[1])`, Sequence{4}},
		{`count( (/root/a/sub)[2]) `, Sequence{1}},
		{`count( /root/sub[position() mod 2 = 0])) `, Sequence{1}},
		{`count( /root/sub[position() mod 2 = 1])) `, Sequence{2}},
		{`empty( () )`, Sequence{true}},
		{`empty( /root/sub )`, Sequence{false}},
		{`empty( /root/doesnotexist )`, Sequence{true}},
		{`empty( /root/@doesnotexist )`, Sequence{true}},
		{`empty( /root/@empty )`, Sequence{false}},
		{`empty( /root/@one )`, Sequence{false}},
		{`floor(1.0)`, Sequence{1.0}},
		{`floor(1.6)`, Sequence{1.0}},
		{`floor(17 div 3)`, Sequence{5.0}},
		{`floor(-3)`, Sequence{-3.0}},
		{`floor(-8.2e0 )`, Sequence{-9.0}},
		{`floor( -0.5e0 )`, Sequence{-1.0}},
		{`string(floor('xxx' ))`, Sequence{"NaN"}},
		{`string(/root/sub[last()])`, Sequence{"contents sub3subsub"}},
		{`/root/local-name()`, Sequence{"root"}},
		{`local-name(/root)`, Sequence{"root"}},
		{`local-name(/)`, Sequence{""}},
		{`/local-name()`, Sequence{""}},
		{`/root/sub/@*[. = 'baz']/local-name()`, Sequence{"foo", "attr"}},
		{`max( (1,2,3) )`, Sequence{3.0}},
		{`max( () )`, Sequence{}},
		{`min( (1,2,3) )`, Sequence{1.0}},
		{`max( () )`, Sequence{}},
		{`normalize-space('  foo bar    baz     ')`, Sequence{"foo bar baz"}},
		{`not( 3 < 6 )`, Sequence{false}},
		{`not( 6 < 3 )`, Sequence{true}},
		{`round(3.2)`, Sequence{3.0}},
		{`round(2.4999)`, Sequence{2.0}},
		{`round(2.5)`, Sequence{3.0}},
		{`round(-7.5)`, Sequence{-7.0}},
		{`round(-7.5001)`, Sequence{-8.0}},
		{`string-join(('a', 'b', 'c'), ', ')`, Sequence{"a, b, c"}},
		{`string-length('a')`, Sequence{1}},
		{`string-length('ä')`, Sequence{1}},
		{`string-length( () )`, Sequence{0}},
		{`upper-case( 'aäÄ' )`, Sequence{"AÄÄ"}},
		{`upper-case( () )`, Sequence{""}},
		{`lower-case( "EΛΛAΣ" )`, Sequence{"eλλaσ"}},
		{`/root/sub[2]/string-length()`, Sequence{4}},
		{`/root/other/string()`, Sequence{"\n\t  contents subsub other\n\t", "\n\t  contents subsub other2\n\t"}},
	}
	doc := `<root empty="" quotationmarks='"text"' one="1" foo="no">
	<sub foo="baz" someattr="somevalue">123</sub>
	<sub foo="bar" attr="baz">sub2</sub>
	<sub foo="bar" self="sub3">contents sub3<subsub foo="bar">subsub</subsub></sub>
	<other foo="barbaz">
	  <subsub foo="oof">contents subsub other</subsub>
	</other>
	<other foo="other2">
	  <subsub foo="oof">contents subsub other2</subsub>
	</other>
	<a>
	<sub p="a1/1"></sub>
	<sub p="a1/2"></sub>
  </a>
  <a>
	<sub  p="a2/1"></sub>
	<sub  p="a2/2"></sub>
  </a>
</root>`
	sr := strings.NewReader(doc)
	np, err := NewParser(sr)
	if err != nil {
		t.Error(err)
	}

	for _, td := range testdata {
		for key, value := range map[string]Sequence{
			"foo":        {"bar"},
			"onedotfive": {1.5},
			"a":          {5.0},
			"two":        {2.0},
			"one":        {1.0},
			"one-two":    {12.0},
		} {
			np.SetVariable(key, value)
		}
		seq, err := np.Evaluate(td.input)
		if err != nil {
			t.Errorf(err.Error())
		}
		if got, want := len(seq), len(td.result); got != want {
			t.Errorf("len(seq) = %d, want %d, test: %s", got, want, td.input)
		}
		for i, itm := range seq {
			if itm != td.result[i] {
				t.Errorf("seq[%d] = %#v, want %v. test: %s", i, itm, td.result[i], td.input)
			}
		}
	}
}
