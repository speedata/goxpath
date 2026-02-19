package goxpath

import (
	"fmt"
	"strings"
	"testing"
)

func generateBenchXML(nChildren, depth int) string {
	var sb strings.Builder
	sb.WriteString("<root>")
	for i := 0; i < nChildren; i++ {
		writeBenchElement(&sb, "sub", depth, i)
	}
	sb.WriteString("</root>")
	return sb.String()
}

func writeBenchElement(sb *strings.Builder, name string, depth, idx int) {
	fmt.Fprintf(sb, `<%s id="%d" class="c%d">`, name, idx, idx%5)
	if depth > 0 {
		for i := 0; i < 3; i++ {
			writeBenchElement(sb, "child", depth-1, idx*10+i)
		}
	} else {
		fmt.Fprintf(sb, "text%d", idx)
	}
	fmt.Fprintf(sb, "</%s>", name)
}

func newBenchParser(b *testing.B, xmlDoc string) *Parser {
	b.Helper()
	np, err := NewParser(strings.NewReader(xmlDoc))
	if err != nil {
		b.Fatal(err)
	}
	return np
}

// BenchmarkTokenize measures tokenization speed.
func BenchmarkTokenize(b *testing.B) {
	cases := []struct {
		name, xpath string
	}{
		{"SimplePath", `/root/sub`},
		{"Predicate", `/root/sub[@foo='bar']`},
		{"Complex", `for $i in /root/sub return concat($i/@foo, '-', string($i))`},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				stringToTokenlist(tc.xpath)
			}
		})
	}
}

// BenchmarkEvaluate measures the full pipeline: tokenize + parse + eval.
func BenchmarkEvaluate(b *testing.B) {
	cases := []struct {
		name, xpath string
	}{
		{"SimplePath", `/root/sub`},
		{"PredicatePos", `/root/sub[2]`},
		{"PredicateAttr", `/root/sub[@foo='bar']`},
		{"DescendantOrSelf", `//sub`},
		{"Ancestor", `/root/sub[3]/subsub/ancestor::element()`},
		{"FollowingSibling", `/root/sub[1]/following-sibling::element()`},
		{"PrecedingSibling", `/root/sub[3]/preceding-sibling::element()`},
		{"ForExpr", `for $i in /root/sub return string($i/@foo)`},
		{"QuantifiedSome", `some $i in /root/sub satisfies $i/@foo="bar"`},
		{"FuncConcat", `concat('hello', ' ', 'world')`},
		{"FuncContains", `contains('Shakespeare', 'spear')`},
		{"FuncStringJoin", `string-join(('a','b','c'), ',')`},
		{"FuncReplace", `replace("banana", "(an)+", "**")`},
		{"FuncMatches", `matches("abracadabra", "bra")`},
		{"FuncTokenize", `tokenize("a,b,c,d", ",")`},
		{"Union", `/root/sub | /root/other`},
		{"Intersect", `/root/sub[position() < 3] intersect /root/sub[@foo='bar']`},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			np := newBenchParser(b, doc)
			np.SetVariable("foo", Sequence{"bar"})
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				np.Evaluate(tc.xpath)
			}
		})
	}
}

// BenchmarkEvalPreParsed measures only evaluation (no re-tokenize/re-parse).
// Comparison with BenchmarkEvaluate shows the parsing overhead.
func BenchmarkEvalPreParsed(b *testing.B) {
	cases := []struct {
		name, xpath string
	}{
		{"SimplePath", `/root/sub`},
		{"PredicateAttr", `/root/sub[@foo='bar']`},
		{"DescendantOrSelf", `//sub`},
		{"ForExpr", `for $i in /root/sub return string($i/@foo)`},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			np := newBenchParser(b, doc)
			tl, err := stringToTokenlist(tc.xpath)
			if err != nil {
				b.Fatal(err)
			}
			ef, err := ParseXPath(tl)
			if err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ef(np.Ctx)
			}
		})
	}
}

// BenchmarkLargeDoc measures performance on a larger XML tree (20 top-level
// elements, depth 3 → ~800 elements total).
func BenchmarkLargeDoc(b *testing.B) {
	largeDoc := generateBenchXML(20, 3)
	cases := []struct {
		name, xpath string
	}{
		{"DescendantAll", `//child`},
		{"DescendantPredicate", `//child[@class='c0']`},
		{"DeepPath", `/root/sub/child/child/child`},
		{"CountDescendant", `count(//child)`},
		{"CountAll", `count(//*)`},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			np := newBenchParser(b, largeDoc)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				np.Evaluate(tc.xpath)
			}
		})
	}
}

// BenchmarkItemStringvalue measures the string conversion hot path.
func BenchmarkItemStringvalue(b *testing.B) {
	b.Run("Float", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			itemStringvalue(3.14159)
		}
	})
	b.Run("Int", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			itemStringvalue(42)
		}
	})
	b.Run("String", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			itemStringvalue("hello world")
		}
	})
}

// BenchmarkCopyContext measures the overhead of context copying,
// which is used in for/some/every/intersect/ancestor expressions.
func BenchmarkCopyContext(b *testing.B) {
	np := newBenchParser(b, doc)
	np.SetVariable("a", Sequence{1.0})
	np.SetVariable("b", Sequence{"hello"})
	np.Ctx.Store = map[interface{}]interface{}{"key": "value"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CopyContext(np.Ctx)
	}
}
