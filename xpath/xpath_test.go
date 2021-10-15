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
		{sequence{item("hello")}, true},
		{sequence{item(1.0)}, true},
		{sequence{item(math.NaN())}, false},
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
