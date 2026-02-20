package goxpath

import (
	"strings"
	"testing"
)

func TestXPathMap(t *testing.T) {
	m := &XPathMap{
		Entries: []MapEntry{
			{Key: "a", Value: Sequence{1.0}},
			{Key: "b", Value: Sequence{2.0}},
			{Key: "c", Value: Sequence{3.0}},
		},
	}

	// Test Get
	val, ok := m.Get("a")
	if !ok {
		t.Error("expected key 'a' to be found")
	}
	if len(val) != 1 || val[0] != 1.0 {
		t.Errorf("Get('a') = %v, want [1.0]", val)
	}

	_, ok = m.Get("z")
	if ok {
		t.Error("expected key 'z' not to be found")
	}

	// Test Keys
	keys := m.Keys()
	if len(keys) != 3 {
		t.Errorf("Keys() length = %d, want 3", len(keys))
	}

	// Test Size
	if got := m.Size(); got != 3 {
		t.Errorf("Size() = %d, want 3", got)
	}

	// Test Contains
	if !m.Contains("b") {
		t.Error("Contains('b') = false, want true")
	}
	if m.Contains("z") {
		t.Error("Contains('z') = true, want false")
	}
}

func TestXPathArray(t *testing.T) {
	arr := &XPathArray{
		Members: []Sequence{
			{1.0},
			{"hello"},
			{true},
		},
	}

	// Test Get (1-based)
	val, err := arr.Get(1)
	if err != nil {
		t.Error(err)
	}
	if len(val) != 1 || val[0] != 1.0 {
		t.Errorf("Get(1) = %v, want [1.0]", val)
	}

	val, err = arr.Get(2)
	if err != nil {
		t.Error(err)
	}
	if len(val) != 1 || val[0] != "hello" {
		t.Errorf("Get(2) = %v, want ['hello']", val)
	}

	// Out of bounds
	_, err = arr.Get(0)
	if err == nil {
		t.Error("Get(0) should return error")
	}
	_, err = arr.Get(4)
	if err == nil {
		t.Error("Get(4) should return error")
	}

	// Test Size
	if got := arr.Size(); got != 3 {
		t.Errorf("Size() = %d, want 3", got)
	}
}

func TestMapFunctions(t *testing.T) {
	testdata := []struct {
		input  string
		result Sequence
	}{
		// map:size
		{`map:size( map { 'a': 1, 'b': 2 } )`, Sequence{2}},
		{`map:size( map { } )`, Sequence{0}},
		// map:keys
		{`count( map:keys( map { 'a': 1, 'b': 2 } ) )`, Sequence{2}},
		// map:contains
		{`map:contains( map { 'a': 1, 'b': 2 }, 'a' )`, Sequence{true}},
		{`map:contains( map { 'a': 1, 'b': 2 }, 'z' )`, Sequence{false}},
		// map:get
		{`map:get( map { 'a': 1, 'b': 2 }, 'a' )`, Sequence{1.0}},
		{`map:get( map { 'a': 1, 'b': 2 }, 'z' )`, Sequence{}},
		// map:put
		{`map:size( map:put( map { 'a': 1 }, 'b', 2 ) )`, Sequence{2}},
		{`map:get( map:put( map { 'a': 1 }, 'a', 99 ), 'a' )`, Sequence{99.0}},
		// map:merge
		{`map:size( map:merge( ( map { 'a': 1 }, map { 'b': 2 } ) ) )`, Sequence{2}},
		// first entry wins in merge
		{`map:get( map:merge( ( map { 'a': 1 }, map { 'a': 99 } ) ), 'a' )`, Sequence{1.0}},
	}

	for _, td := range testdata {
		sr := strings.NewReader(doc)
		np, err := NewParser(sr)
		if err != nil {
			t.Error(err)
			continue
		}
		seq, err := np.Evaluate(td.input)
		if err != nil {
			t.Errorf("error evaluating %q: %v", td.input, err)
			continue
		}
		if got, want := len(seq), len(td.result); got != want {
			t.Errorf("len(seq) = %d, want %d, test: %s (got %v)", got, want, td.input, seq)
			continue
		}
		for i, itm := range seq {
			if itm != td.result[i] {
				t.Errorf("seq[%d] = %#v, want %#v. test: %s", i, itm, td.result[i], td.input)
			}
		}
	}
}

func TestArrayFunctions(t *testing.T) {
	testdata := []struct {
		input  string
		result Sequence
	}{
		// array constructor and array:size
		{`array:size( array { 1, 2, 3 } )`, Sequence{3}},
		{`array:size( array { } )`, Sequence{0}},
		// array:get
		{`array:get( array { 10, 20, 30 }, 1 )`, Sequence{10.0}},
		{`array:get( array { 10, 20, 30 }, 2 )`, Sequence{20.0}},
		{`array:get( array { 10, 20, 30 }, 3 )`, Sequence{30.0}},
		// array with strings
		{`array:get( array { 'hello', 'world' }, 2 )`, Sequence{"world"}},
	}

	for _, td := range testdata {
		sr := strings.NewReader(doc)
		np, err := NewParser(sr)
		if err != nil {
			t.Error(err)
			continue
		}
		seq, err := np.Evaluate(td.input)
		if err != nil {
			t.Errorf("error evaluating %q: %v", td.input, err)
			continue
		}
		if got, want := len(seq), len(td.result); got != want {
			t.Errorf("len(seq) = %d, want %d, test: %s (got %v)", got, want, td.input, seq)
			continue
		}
		for i, itm := range seq {
			if itm != td.result[i] {
				t.Errorf("seq[%d] = %#v, want %#v. test: %s", i, itm, td.result[i], td.input)
			}
		}
	}
}

func TestMapConstructorWithVariables(t *testing.T) {
	sr := strings.NewReader(doc)
	np, err := NewParser(sr)
	if err != nil {
		t.Fatal(err)
	}
	np.SetVariable("mymap", Sequence{&XPathMap{
		Entries: []MapEntry{
			{Key: "x", Value: Sequence{42.0}},
		},
	}})

	seq, err := np.Evaluate(`map:get($mymap, 'x')`)
	if err != nil {
		t.Fatal(err)
	}
	if len(seq) != 1 || seq[0] != 42.0 {
		t.Errorf("got %v, want [42.0]", seq)
	}
}

func TestArrayConstructorWithVariables(t *testing.T) {
	sr := strings.NewReader(doc)
	np, err := NewParser(sr)
	if err != nil {
		t.Fatal(err)
	}
	np.SetVariable("myarr", Sequence{&XPathArray{
		Members: []Sequence{
			{100.0},
			{200.0},
		},
	}})

	seq, err := np.Evaluate(`array:get($myarr, 2)`)
	if err != nil {
		t.Fatal(err)
	}
	if len(seq) != 1 || seq[0] != 200.0 {
		t.Errorf("got %v, want [200.0]", seq)
	}

	seq, err = np.Evaluate(`array:size($myarr)`)
	if err != nil {
		t.Fatal(err)
	}
	if len(seq) != 1 || seq[0] != 2 {
		t.Errorf("got %v, want [2]", seq)
	}
}
