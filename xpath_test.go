package goxpath

import (
	"math"
	"strings"
	"testing"
	"time"
)

var nsDoc = `<a:root xmlns:a="anamespace">
  <a:sub>text</a:sub>
</a:root>`

var doc = `<root empty="" quotationmarks='"text"' one="1" foo="no">
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
	currentTimeGetter = func() time.Time {
		return time.Unix(1700558398, 0)
	}
	testdata := []struct {
		input  string
		result Sequence
	}{
		{`string(xs:time("11:23:00")) `, Sequence{"11:23:00.000+00:00"}},
		{`string-to-codepoints( "hellö" ) `, Sequence{104, 101, 108, 108, 246}},
		{`codepoints-to-string( (65,33*2,67) )`, Sequence{"ABC"}},
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
		{`/root/@one < 2 and /root/@one >= 1`, Sequence{true}},
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
		{`boolean(/root) `, Sequence{true}},
		{`ceiling(1.0)`, Sequence{1.0}},
		{`ceiling(1.6)`, Sequence{2.0}},
		{`ceiling(17 div 3)`, Sequence{6.0}},
		{`ceiling(-3)`, Sequence{-3.0}},
		{`ceiling(-8.2e0 )`, Sequence{-8.0e0}},
		{`ceiling( -0.5e0 )`, Sequence{-0.0}},
		{`string(ceiling('xxx' ))`, Sequence{"NaN"}},
		{`codepoints-to-string( reverse(  string-to-codepoints('Hellö')  ) ) `, Sequence{"ölleH"}},
		{`codepoint-equal('ö','ö') `, Sequence{false}},
		{`codepoint-equal('ö','ö') `, Sequence{true}},
		{`codepoint-equal((),'ö') `, Sequence{}},
		{`compare('abc', 'abc')  `, Sequence{0}},
		{`compare('def', 'abc')  `, Sequence{1}},
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
		{`contains((), "a")`, Sequence{false}},
		{`contains("", "")`, Sequence{true}},
		{`contains("Shakespeare", "")`, Sequence{true}},
		{`contains("", "a")`, Sequence{false}},
		{`contains("Shakespeare", "spear")`, Sequence{true}},
		{`string(current-dateTime())  `, Sequence{"2023-11-21T10:19:58.000+01:00"}},
		{`string(current-date())  `, Sequence{"2023-11-21+01:00"}},
		{`string(current-time())  `, Sequence{"10:19:58.000+01:00"}},
		{`empty( () )`, Sequence{true}},
		{`empty( /root/sub )`, Sequence{false}},
		{`empty( /root/doesnotexist )`, Sequence{true}},
		{`empty( /root/@doesnotexist )`, Sequence{true}},
		{`empty( /root/@empty )`, Sequence{false}},
		{`empty( /root/@one )`, Sequence{false}},
		{`ends-with("tattoo", "too")   `, Sequence{true}},
		{`ends-with("tattoo", "ott")   `, Sequence{false}},
		{`floor(1.0)`, Sequence{1.0}},
		{`floor(1.6)`, Sequence{1.0}},
		{`floor(17 div 3)`, Sequence{5.0}},
		{`floor(-3)`, Sequence{-3.0}},
		{`floor(-8.2e0 )`, Sequence{-9.0}},
		{`floor( -0.5e0 )`, Sequence{-1.0}},
		{`string(floor('xxx' ))`, Sequence{"NaN"}},
		{`string(/root/sub[last()])`, Sequence{"contents sub3subsub"}},
		{`/root/local-name()`, Sequence{"root"}},
		{`local-name(root()/*)`, Sequence{"root"}},
		{`hours-from-time(xs:time("11:23:00")) `, Sequence{"11"}},
		{`minutes-from-time(xs:time("11:23:00")) `, Sequence{"23"}},
		{`seconds-from-time(xs:time("11:23:01")) `, Sequence{"01"}},
		{`local-name(/root)`, Sequence{"root"}},
		{`local-name(/)`, Sequence{""}},
		{`/local-name()`, Sequence{""}},
		{`/root/sub/@*[. = 'baz']/local-name()`, Sequence{"foo", "attr"}},
		{`matches("abracadabra", "bra")`, Sequence{true}},
		{`matches("banana", "^(.a)+$")`, Sequence{true}},
		{`matches("", "a*")`, Sequence{true}},
		{`matches("23 May 2008", "^[0-9]+\s[A-Z][a-z]+\s[0-9]+$")`, Sequence{true}},
		{`max( (1,2,3) )`, Sequence{3.0}},
		{`max( () )`, Sequence{}},
		{`min( (1,2,3) )`, Sequence{1.0}},
		{`max( () )`, Sequence{}},
		{`normalize-space('  foo bar    baz     ')`, Sequence{"foo bar baz"}},
		{`not( 3 < 6 )`, Sequence{false}},
		{`not( 6 < 3 )`, Sequence{true}},
		{`replace("facetiously", "[aeiouy]", "[$0]")`, Sequence{"f[a]c[e]t[i][o][u]sl[y]"}},
		{`replace("banana", "(an)+?", "**")`, Sequence{"b****a"}},
		{`replace("banana", "(an)+", "**")`, Sequence{"b**a"}},
		{`replace("banana", "(ana|na)", "[$1]")`, Sequence{"b[ana][na]"}},
		{`replace("banana", "a", "o")`, Sequence{"bonono"}},
		{`reverse( ( 1,2,3  ) ) `, Sequence{3.0, 2.0, 1.0}},
		{`round(3.2)`, Sequence{3.0}},
		{`round(2.4999)`, Sequence{2.0}},
		{`round(2.5)`, Sequence{3.0}},
		{`round(-7.5)`, Sequence{-7.0}},
		{`round(-7.5001)`, Sequence{-8.0}},
		{`starts-with("tattoo", "tat")   `, Sequence{true}},
		{`starts-with("tattoo", "att")   `, Sequence{false}},
		{`string-join(('a', 'b', 'c'), ', ')`, Sequence{"a, b, c"}},
		{`string-length('a')`, Sequence{1}},
		{`string-length('ä')`, Sequence{1}},
		{`string-length( () )`, Sequence{0}},
		{`substring("öäü", 2) `, Sequence{"äü"}},
		{`substring("Goldfarb", 5) `, Sequence{"farb"}},
		{`substring("Goldfarb", 5, 3) `, Sequence{"far"}},
		{`substring-after("tattoo", "tat")  `, Sequence{"too"}},
		{`substring-after ( "tattoo", "tattoo") `, Sequence{""}},
		{`substring-before("tattoo", "attoo")  `, Sequence{"t"}},
		{`substring-before ( "tattoo", "tatto") `, Sequence{""}},
		{`substring-before ( (), ()) `, Sequence{""}},
		{`tokenize("12, 16, 2", ",\s*")"`, Sequence{"12", "16", "2"}},
		{`tokenize("abc[NL]def[XY]", "\[.*?\]")"`, Sequence{"abc", "def", ""}},
		{`tokenize("Go home, Jack!","\W+")`, Sequence{"Go", "home", "Jack", ""}},
		{`translate("bar","abc","ABC")  `, Sequence{"BAr"}},
		{`translate("--aaa--","abc-","ABC")  `, Sequence{"AAA"}},
		{`translate("abcdabc", "abc", "AB")  `, Sequence{"ABdAB"}},
		{`upper-case( 'aäÄ' )`, Sequence{"AÄÄ"}},
		{`upper-case( () )`, Sequence{""}},
		{`lower-case( "EΛΛAΣ" )`, Sequence{"eλλaσ"}},
		{`/root/sub[2]/string-length()`, Sequence{4}},
		{`/root/other/string()`, Sequence{"\n\t  contents subsub other\n\t", "\n\t  contents subsub other2\n\t"}},
		{`count(/root/descendant-or-self::sub) `, Sequence{7}},
		{`/child::root/@foo = 'no'`, Sequence{true}},
		{`count(/root/sub/descendant-or-self::sub)  `, Sequence{3}},
		{`/root/sub/text()  `, Sequence{"123", "sub2", "contents sub3"}},
		{`count(/root/sub/descendant-or-self::text())  `, Sequence{4}},
		{`/root/sub/descendant-or-self::text()[2])  `, Sequence{"subsub"}},
		{`(/root/*/descendant::sub/@p)[4] = "a2/2"  `, Sequence{true}},
		{`count(/root/*/descendant::sub[1]) `, Sequence{2}},
		{`count( /root/sub[3]/following-sibling::element() )`, Sequence{4}},
		{`count(/root/a/node())  `, Sequence{10}},
		{`(/root/a/node()[2]/@p)[1] = 'a1/1'  `, Sequence{true}},
		{`count(/root//sub)  `, Sequence{7}},
		{`count(/root//sub[1])  `, Sequence{3}},
		{`count(/root//text())  `, Sequence{24}},
		{`/root/sub[1]/attribute::*[1]/string() `, Sequence{"baz"}},
		{`count(/root/child::element())  `, Sequence{7}},
		{`local-name( (/root/sub[3]/following-sibling::element())[2])  `, Sequence{"other"}},
		{`count( /root/sub[3]/following-sibling::element() )  `, Sequence{4}},
		{`count(/root/sub[3]/following::element() ) `, Sequence{10}},
		{`local-name( (/root/sub[3]/following-sibling::element())[2])  `, Sequence{"other"}},
		{`count( /root/sub[3]/following-sibling::element() )  `, Sequence{4}},
		{`count(/root/sub[3]/following::element() )  `, Sequence{10}},
		{`/root/sub[3]/subsub/parent::element()/local-name()   `, Sequence{"sub"}},
		{`count(/root/sub[3]/subsub/ancestor::element())    `, Sequence{2}},
		{`/root/sub[3]/subsub/ancestor::element()/local-name()    `, Sequence{"root", "sub"}},
		{`/root/sub[3]/subsub/ancestor-or-self::element()/local-name() `, Sequence{"root", "sub", "subsub"}},
		{`/root/sub[3]/preceding-sibling::element()/string(@foo) `, Sequence{"baz", "bar"}},
		{`/root/other[1]/preceding::element()/string() `, Sequence{"123", "sub2", "contents sub3subsub", "subsub"}},
		{`/root//subsub[1]/../@self = "sub3" `, Sequence{true}},
		{`for $i in 1 to 2 , $j in 2 to 3 return $i * $j `, Sequence{2.0, 3.0, 4.0, 6.0}},
		{`count ( for $i in /root/sub return $i ) `, Sequence{3}},
		{`some  $i in (1,2) satisfies $i = 1  `, Sequence{true}},
		{`every $i in (1,2) satisfies $i = 1  `, Sequence{false}},
		{`every $i in (1,2) satisfies $i = (1,2)  `, Sequence{true}},
		{`for $i in /root/sub return $i[1]/@foo/string() `, Sequence{"baz", "bar", "bar"}},
		{`for $i in /root/* return $i/local-name() `, Sequence{"sub", "sub", "sub", "other", "other", "a", "a"}},
		{`some $i in /root/sub satisfies $i/@foo="bar" `, Sequence{true}},
		{`some $x in (1, 2, 3), $y in (2, 3) satisfies $x + $y = 4 `, Sequence{true}},
		{`every $x in (1, 2, 3), $y in (2, 3) satisfies $x + $y = 4 `, Sequence{false}},
		{`every $x in 1, $y in 2 satisfies $x + $y = 3 `, Sequence{true}},
		{`every $x in /root/sub, $y in 2 satisfies not(empty($x/@foo)) `, Sequence{true}},
		{`every $x in /root/@one, $y in 2 satisfies $x + $y = 3.0 `, Sequence{true}},
		{`every $x in /root/@one, $y in 2 satisfies $x + $y + $a = 8.0 `, Sequence{true}},
		{`/root/sub[1] is /root/sub[2]/preceding-sibling::sub[1]`, Sequence{true}},
		{`/root/sub[1] << /root/sub[@self='sub3']`, Sequence{true}},
		{`/root/sub[2] >> /root/sub[1]`, Sequence{true}},
		{`/root/sub[1] << /root/sub[4]`, Sequence{}},
		{`string(/root/sub[position() < 3] intersect /root/sub[@foo='bar']) `, Sequence{"sub2"}},
		{`string(for $seq1 in /root/sub[position() < 3], $seq2 in /root/sub[@foo='bar'] return $seq1 intersect $seq2)`, Sequence{"sub2"}},
		{`string(/root/sub[position() < 3] except /root/sub[@foo='bar']) `, Sequence{"123"}},
		{`( /root/sub )/string()`, Sequence{"123", "sub2", "contents sub3subsub"}},
		{`(/root/sub[position() < 3] except /root/sub[@foo='bar'])/string()`, Sequence{"123"}},
		{`count( //sub ) `, Sequence{7}},
		{`count(/root/sub[3][1]) `, Sequence{1}},
		{`/root/sub instance of empty-sequence() `, Sequence{false}},
		{`/root/sub instance of empty-sequence() `, Sequence{false}},
		{`/root/sub[4] instance of empty-sequence() `, Sequence{true}},
		{`/root/sub[1]/attribute()/string() `, Sequence{"baz", "somevalue"}},
		{`/root/sub[1]/attribute(*)/string() `, Sequence{"baz", "somevalue"}},
		{`/root/sub[1]/attribute(foo)/string() `, Sequence{"baz"}},
		{`text`, Sequence{}},
	}

	for _, td := range testdata {
		sr := strings.NewReader(doc)
		np, err := NewParser(sr)
		if err != nil {
			t.Error(err)
		}

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
				t.Errorf("seq[%d] = %#v, want %#v. test: %s", i, itm, td.result[i], td.input)
			}
		}
	}
}

func TestNSEval(t *testing.T) {
	currentTimeGetter = func() time.Time {
		return time.Unix(1700558398, 0)
	}
	testdata := []struct {
		input  string
		result Sequence
	}{
		{`string(/a:root/a:sub)`, Sequence{"text"}},
	}
	for _, td := range testdata {
		sr := strings.NewReader(nsDoc)
		np, err := NewParser(sr)
		np.Ctx.Namespaces["a"] = "anamespace"
		if err != nil {
			t.Error(err)
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
				t.Errorf("seq[%d] = %#v, want %#v. test: %s", i, itm, td.result[i], td.input)
			}
		}
	}
}
