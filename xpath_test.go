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

var doc = `<!-- comment --><?pi text ?><root empty="" quotationmarks='"text"' one="1" foo="no">
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
		{`string(data(/root/sub[1]))`, Sequence{"123"}},
		{`data( (1, 'hello', 3.5) )`, Sequence{1.0, "hello", 3.5}},
		{`day-from-date(xs:date("2004-05-12"))`, Sequence{12}},
		{`day-from-dateTime(xs:dateTime("2004-05-12T18:17:15"))`, Sequence{12}},
		{`days-from-duration(xs:duration("P3DT10H"))`, Sequence{3}},
		{`days-from-duration(xs:duration("-P3DT10H"))`, Sequence{-3}},
		{`hours-from-duration(xs:duration("P3DT10H"))`, Sequence{10}},
		{`hours-from-duration(xs:duration("-PT10H"))`, Sequence{-10}},
		{`minutes-from-duration(xs:duration("P3DT10H30M"))`, Sequence{30}},
		{`months-from-duration(xs:duration("P1Y6M"))`, Sequence{6}},
		{`months-from-duration(xs:duration("-P1Y6M"))`, Sequence{-6}},
		{`seconds-from-duration(xs:duration("PT1M30.5S"))`, Sequence{30.5}},
		{`years-from-duration(xs:duration("P3Y6M"))`, Sequence{3}},
		{`years-from-duration(xs:duration("-P3Y"))`, Sequence{-3}},
		{`year-from-date(xs:date("2004-05-12"))`, Sequence{2004}},
		{`year-from-dateTime(xs:dateTime("2004-05-12T18:17:15"))`, Sequence{2004}},
		{`month-from-date(xs:date("2004-05-12"))`, Sequence{5}},
		{`month-from-dateTime(xs:dateTime("2004-05-12T18:17:15"))`, Sequence{5}},
		{`hours-from-dateTime(xs:dateTime("2004-05-12T18:17:15"))`, Sequence{18}},
		{`minutes-from-dateTime(xs:dateTime("2004-05-12T18:17:15"))`, Sequence{17}},
		{`seconds-from-dateTime(xs:dateTime("2004-05-12T18:17:15"))`, Sequence{15.0}},
		{`encode-for-uri("http://example.com/~bébé")`, Sequence{"http%3A%2F%2Fexample.com%2F~b%C3%A9b%C3%A9"}},
		{`encode-for-uri("100% organic")`, Sequence{"100%25%20organic"}},
		{`encode-for-uri("")`, Sequence{""}},
		{`deep-equal( (1, 2, 3), (1, 2, 3) )`, Sequence{true}},
		{`deep-equal( (1, 2, 3), (1, 2) )`, Sequence{false}},
		{`deep-equal( ('a', 'b'), ('a', 'b') )`, Sequence{true}},
		{`deep-equal( ('a', 'b'), ('a', 'c') )`, Sequence{false}},
		{`deep-equal( (), () )`, Sequence{true}},
		{`ends-with("tattoo", "too")   `, Sequence{true}},
		{`ends-with("tattoo", "ott")   `, Sequence{false}},
		{`exactly-one( ('a') )`, Sequence{"a"}},
		{`floor(1.0)`, Sequence{1.0}},
		{`floor(1.6)`, Sequence{1.0}},
		{`floor(17 div 3)`, Sequence{5.0}},
		{`floor(-3)`, Sequence{-3.0}},
		{`floor(-8.2e0 )`, Sequence{-9.0}},
		{`floor( -0.5e0 )`, Sequence{-1.0}},
		{`escape-html-uri("http://example.com/test#car")`, Sequence{"http://example.com/test#car"}},
		{`escape-html-uri("javascript:if (navigator.browserLanguage == 'fr') window.open('http://example.com/français');")`, Sequence{"javascript:if (navigator.browserLanguage == 'fr') window.open('http://example.com/fran%C3%A7ais');"}},
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
		{`min( () )`, Sequence{}},
		{`sum( (1,2,3) )`, Sequence{6.0}},
		{`sum( () )`, Sequence{0.0}},
		{`sum( (1.5, 2.5, 3.0) )`, Sequence{7.0}},
		{`avg( (1,2,3) )`, Sequence{2.0}},
		{`avg( () )`, Sequence{}},
		{`avg( (10, 20) )`, Sequence{15.0}},
		{`exists( () )`, Sequence{false}},
		{`exists( (1,2) )`, Sequence{true}},
		{`exists( /root/sub )`, Sequence{true}},
		{`exists( /root/doesnotexist )`, Sequence{false}},
		{`distinct-values( (1, 2, 1, 3, 2) )`, Sequence{1.0, 2.0, 3.0}},
		{`distinct-values( ('a', 'b', 'a') )`, Sequence{"a", "b"}},
		{`distinct-values( () )`, Sequence{}},
		{`subsequence( (1, 2, 3, 4), 2 )`, Sequence{2.0, 3.0, 4.0}},
		{`subsequence( (1, 2, 3, 4), 2, 2 )`, Sequence{2.0, 3.0}},
		{`subsequence( (1, 2, 3), 1, 1 )`, Sequence{1.0}},
		{`subsequence( (), 1 )`, Sequence{}},
		{`subsequence( (1, 2, 3), 10 )`, Sequence{}},
		{`index-of( (1, 2, 3, 2, 1), 2 )`, Sequence{2, 4}},
		{`index-of( ('a', 'b', 'c'), 'b' )`, Sequence{2}},
		{`index-of( (1, 2, 3), 5 )`, Sequence{}},
		{`index-of( (), 1 )`, Sequence{}},
		{`insert-before( ('a', 'b', 'c'), 2, 'x' )`, Sequence{"a", "x", "b", "c"}},
		{`insert-before( ('a', 'b', 'c'), 1, 'x' )`, Sequence{"x", "a", "b", "c"}},
		{`insert-before( ('a', 'b', 'c'), 10, 'x' )`, Sequence{"a", "b", "c", "x"}},
		{`insert-before( (), 1, 'x' )`, Sequence{"x"}},
		{`format-number( 1234.5, '#,##0.00' )`, Sequence{"1,234.50"}},
		{`format-number( 1234567, '#,###' )`, Sequence{"1,234,567"}},
		{`format-number( 0.5, '0.00' )`, Sequence{"0.50"}},
		{`format-number( -1234, '#,##0' )`, Sequence{"-1,234"}},
		{`iri-to-uri("http://www.example.com/~bébé")`, Sequence{"http://www.example.com/~b%C3%A9b%C3%A9"}},
		{`iri-to-uri("http://example.com/path")`, Sequence{"http://example.com/path"}},
		{`normalize-space('  foo bar    baz     ')`, Sequence{"foo bar baz"}},
		{`normalize-unicode("ä")`, Sequence{"\u00e4"}},
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
		{`round-half-to-even(0.5)`, Sequence{0.0}},
		{`round-half-to-even(1.5)`, Sequence{2.0}},
		{`round-half-to-even(2.5)`, Sequence{2.0}},
		{`round-half-to-even(3.567812e+3, 2)`, Sequence{3567.81}},
		{`round-half-to-even(4.7564e-3, 2)`, Sequence{0.0}},
		{`round-half-to-even(35612.25, -2)`, Sequence{35600.0}},
		{`one-or-more( (1, 2) )`, Sequence{1.0, 2.0}},
		{`one-or-more( ('a') )`, Sequence{"a"}},
		{`remove( ('a', 'b', 'c'), 2 )`, Sequence{"a", "c"}},
		{`remove( ('a', 'b', 'c'), 1 )`, Sequence{"b", "c"}},
		{`remove( ('a', 'b', 'c'), 0 )`, Sequence{"a", "b", "c"}},
		{`remove( (), 1 )`, Sequence{}},
		{`unordered( (1, 2, 3) )`, Sequence{1.0, 2.0, 3.0}},
		{`zero-or-one( () )`, Sequence{}},
		{`zero-or-one( ('a') )`, Sequence{"a"}},
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
		{`string(QName("http://example.com/ns", "foo:bar"))`, Sequence{"foo:bar"}},
		{`string(QName("", "bar"))`, Sequence{"bar"}},
		{`local-name-from-QName(QName("http://example.com/ns", "foo:bar"))`, Sequence{"bar"}},
		{`prefix-from-QName(QName("http://example.com/ns", "foo:bar"))`, Sequence{"foo"}},
		{`prefix-from-QName(QName("", "bar"))`, Sequence{}},
		{`namespace-uri-from-QName(QName("http://example.com/ns", "foo:bar"))`, Sequence{"http://example.com/ns"}},
		{`namespace-uri-from-QName(QName("", "bar"))`, Sequence{""}},
		{`text`, Sequence{}},
		{`count(/)`, Sequence{1}},
		{`string(/comment())`, Sequence{" comment "}},
		{`string(/processing-instruction())`, Sequence{"text "}},
		{`string(/processing-instruction(pi))`, Sequence{"text "}},
		{`string(/processing-instruction(doesnotexist))`, Sequence{""}},
		{`string(adjust-dateTime-to-timezone(xs:dateTime("2002-03-07T10:00:00-05:00"), xs:duration("PT0S")))`, Sequence{"2002-03-07T15:00:00.000+00:00"}},
		{`string(adjust-dateTime-to-timezone(xs:dateTime("2002-03-07T10:00:00-05:00"), xs:duration("-PT10H")))`, Sequence{"2002-03-07T05:00:00.000-10:00"}},
		{`string(adjust-dateTime-to-timezone(xs:dateTime("2002-03-07T10:00:00-05:00"), xs:duration("PT5H30M")))`, Sequence{"2002-03-07T20:30:00.000+05:30"}},
		{`string(adjust-date-to-timezone(xs:date("2002-03-07-05:00"), xs:duration("PT0S")))`, Sequence{"2002-03-07+00:00"}},
		{`string(adjust-time-to-timezone(xs:time("10:00:00-05:00"), xs:duration("PT0S")))`, Sequence{"15:00:00.000+00:00"}},
		{`doc-available("nonexistent-file-xyz.xml")`, Sequence{false}},
		{`resolve-uri("bar", "http://example.com/foo/")`, Sequence{"http://example.com/foo/bar"}},
		{`resolve-uri("../bar", "http://example.com/foo/baz")`, Sequence{"http://example.com/bar"}},
		{`resolve-uri("http://example.com/abs", "http://other.com/")`, Sequence{"http://example.com/abs"}},
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
			t.Error(err)
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

func TestLang(t *testing.T) {
	langDoc := `<root xml:lang="en">
  <p>English</p>
  <div xml:lang="de">
    <p>German</p>
  </div>
</root>`
	testdata := []struct {
		input  string
		result Sequence
	}{
		{`lang("en", /root)`, Sequence{true}},
		{`lang("en", /root/p)`, Sequence{true}},
		{`lang("EN", /root)`, Sequence{true}},
		{`lang("de", /root/div)`, Sequence{true}},
		{`lang("de", /root/div/p)`, Sequence{true}},
		{`lang("en", /root/div)`, Sequence{false}},
		{`lang("fr", /root)`, Sequence{false}},
	}
	for _, td := range testdata {
		sr := strings.NewReader(langDoc)
		np, err := NewParser(sr)
		if err != nil {
			t.Fatal(err)
		}
		seq, err := np.Evaluate(td.input)
		if err != nil {
			t.Errorf("error evaluating %s: %v", td.input, err)
			continue
		}
		if got, want := len(seq), len(td.result); got != want {
			t.Errorf("len(seq) = %d, want %d, test: %s", got, want, td.input)
			continue
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
		{`local-name-from-QName(resolve-QName("a:sub", /a:root))`, Sequence{"sub"}},
		{`namespace-uri-from-QName(resolve-QName("a:sub", /a:root))`, Sequence{"anamespace"}},
		{`namespace-uri-for-prefix("a", /a:root)`, Sequence{"anamespace"}},
		{`namespace-uri-for-prefix("xml", /a:root)`, Sequence{"http://www.w3.org/XML/1998/namespace"}},
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
			t.Error(err)
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
