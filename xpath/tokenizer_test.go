package xpath

import (
	"strings"
	"testing"
)

func TestNCName(t *testing.T) {
	testdata := []struct {
		input  token
		output bool
	}{
		{token{TokNumber, 1}, false},
	}
	for _, td := range testdata {
		got := td.input.isNCName()
		if expected := td.output; got != expected {
			t.Errorf("%v isNCName() = %t, want %t", td.input, got, expected)
		}
	}
}

func TestGetQName(t *testing.T) {
	testdata := []struct {
		input  string
		output string
	}{
		{"hello", "hello"},
		{"abc*def", "abc"},
		{"abc:def", "abc:def"},
		{"abc:def:ghi", "abc:def"},
		{"abc_def", "abc_def"},
		{"abc-def", "abc-def"},
		{"abc·def", "abc·def"},
		{"abc‿def", "abc‿def"},
		{"a123", "a123"},
	}
	for _, td := range testdata {
		sr := strings.NewReader(td.input)
		res, err := getQName(sr)
		if err != nil {
			t.Error(err.Error())
		}
		if got, expected := res, td.output; got != expected {
			t.Errorf("getWord(%s) = %s, want %s", td.input, res, expected)
		}
	}
}

func TestGetDelimitedString(t *testing.T) {
	testdata := []struct {
		input  string
		output string
	}{
		{`"hello"`, `hello`},
		{`'hello'`, `hello`},
		{`'he"llo'`, `he"llo`},
		{`"he'llo"`, `he'llo`},
	}
	for _, td := range testdata {
		sr := strings.NewReader(td.input)
		res, err := getDelimitedString(sr)
		if err != nil {
			t.Error(err.Error())
		}
		if got, expected := res, td.output; got != expected {
			t.Errorf("getDelimitedString(%s) = %s, want %s", td.input, res, expected)
		}
	}
}
func TestGetAxis(t *testing.T) {
	testdata := []struct {
		input  string
		output []token
	}{
		{`child::sub`, []token{{"child", TokDoubleColon}, {"sub", TokQName}}},
	}
	for _, td := range testdata {
		toklist, err := stringToTokenlist(td.input)
		if err != nil {
			t.Error(err)
		}
		toks := toklist.toks
		if len(toks) != len(td.output) {
			t.Errorf("len(toks) = %d (%v), want %d (%v)", len(toks), toks, len(td.output), td.output)
		} else {
			for i, tok := range toks {
				expected := td.output[i]
				if tok.Typ != expected.Typ || tok.Value != expected.Value {
					t.Errorf("tok[%d] = %v, want %v", i, tok, expected)
				}
			}
		}
	}
}

func TestOperator(t *testing.T) {
	testdata := []struct {
		input  string
		output []token
	}{
		{`< (:comment (:nested :) :) `, []token{{`<`, TokOperator}}},
		{`"hello"`, []token{{"hello", TokString}}},
		{`'hello'`, []token{{"hello", TokString}}},
		{`< `, []token{{`<`, TokOperator}}},
		{`<= `, []token{{`<=`, TokOperator}}},
		{`> `, []token{{`>`, TokOperator}}},
		{`>= `, []token{{`>=`, TokOperator}}},
		{`!= `, []token{{`!=`, TokOperator}}},
		{`<< `, []token{{`<<`, TokOperator}}},
		{`>> `, []token{{`>>`, TokOperator}}},
		{`/ `, []token{{`/`, TokOperator}}},
		{`// `, []token{{`//`, TokOperator}}},
		{`: `, []token{{`:`, TokOperator}}},
		{`:: `, []token{{`::`, TokOperator}}},
		{`.`, []token{{`.`, TokOperator}}},
		{`(1,2)`, []token{{'(', TokOpenParen}, {1.0, TokNumber}, {`,`, TokComma}, {2.0, TokNumber}, {')', TokCloseParen}}},
		{`$hello`, []token{{"hello", TokVarname}}},
	}
	for _, td := range testdata {
		toklist, err := stringToTokenlist(td.input)
		if err != nil {
			t.Error(err)
		}
		toks := toklist.toks
		if len(toks) != len(td.output) {
			t.Errorf("len(toks) = %d (%v), want %d (%v)", len(toks), toks, len(td.output), td.output)
		} else {
			for i, tok := range toks {
				expected := td.output[i]
				if tok.Typ != expected.Typ || tok.Value != expected.Value {
					t.Errorf("tok[%d] = %v, want %v", i, tok, expected)
				}
			}
		}
	}
}
