package goxpath

import (
	"fmt"
	"strings"
	"testing"
)

// TestFocusPreserved checks that evaluating a path does not change the
// context position and size of the surrounding focus (issue #9). A host
// that loops over items sets the focus and evaluates expressions for
// each item, like xsl:for-each does.
func TestFocusPreserved(t *testing.T) {
	p, err := NewParser(strings.NewReader("<root><rec/><rec/><rec/><rec/><rec/><x/><x/></root>"))
	if err != nil {
		t.Fatal(err)
	}
	recs, err := p.Evaluate("/root/rec")
	if err != nil {
		t.Fatal(err)
	}
	testdata := []struct {
		expr  string
		count int
	}{
		{"count(following-sibling::x)", 2},
		{"count(../x)", 2},
		{"count(..//x)", 2},
		{"count(../x[1])", 1},
		{"count(following-sibling::*[last()])", 1},
		{"count((../x)[1])", 1},
		{"count((1, 2, 3)[. > 1])", 2},
		{"count(../x ! .)", 2},
	}
	for _, td := range testdata {
		xpath := "concat(" + td.expr + ", ' ', position(), '/', last())"
		for i, r := range recs {
			p.Ctx.SetContextSequence(Sequence{r})
			p.Ctx.Pos = i + 1
			p.Ctx.SetSize(len(recs))
			seq, err := p.Evaluate(xpath)
			if err != nil {
				t.Fatalf("%s: %v", xpath, err)
			}
			want := fmt.Sprintf("%d %d/%d", td.count, i+1, len(recs))
			if got := seq.Stringvalue(); got != want {
				t.Errorf("%s at position %d: got %q, want %q", xpath, i+1, got, want)
			}
			if p.Ctx.Pos != i+1 || p.Ctx.Size() != len(recs) {
				t.Errorf("%s at position %d: focus after Evaluate is %d/%d, want %d/%d",
					xpath, i+1, p.Ctx.Pos, p.Ctx.Size(), i+1, len(recs))
			}
		}
	}
}

func TestSimpleMapPosition(t *testing.T) {
	p, err := NewParser(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatal(err)
	}
	seq, err := p.Evaluate("string-join(('a', 'b', 'c') ! (position() || '/' || last()), ' ')")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := seq.Stringvalue(), "1/3 2/3 3/3"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
