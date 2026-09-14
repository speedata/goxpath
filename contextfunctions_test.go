package goxpath

import (
	"strings"
	"testing"
)

// TestContextRegisterFunction verifies that a function registered on a context
// is visible to that evaluation only, leaks neither into the global registry nor
// into an unrelated parser, and survives the context copies that evaluation
// makes internally (a for expression copies the context per iteration).
func TestContextRegisterFunction(t *testing.T) {
	np, err := NewParser(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	np.Ctx.RegisterFunction(&Function{
		Name:      "local-only",
		Namespace: nsFN,
		F: func(*Context, []Sequence) (Sequence, error) {
			return Sequence{"here"}, nil
		},
	})

	seq, err := np.Evaluate(`local-only()`)
	if err != nil {
		t.Fatalf("local-only(): %v", err)
	}
	if got := seq.Stringvalue(); got != "here" {
		t.Errorf("local-only() = %q, want %q", got, "here")
	}
	if !np.Ctx.FunctionExists(nsFN, "local-only") {
		t.Error("Context.FunctionExists does not see the registered function")
	}

	// Inside a for expression, so the call happens in a copied context.
	seq, err = np.Evaluate(`string-join(for $i in (1, 2) return local-only(), "-")`)
	if err != nil {
		t.Fatalf("local-only() in a for expression: %v", err)
	}
	if got := seq.Stringvalue(); got != "here-here" {
		t.Errorf("local-only() in a for expression = %q, want %q", got, "here-here")
	}

	if FunctionExists(nsFN, "local-only") {
		t.Error("the function leaked into the global registry")
	}
	other, err := NewParser(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Evaluate(`local-only()`); err == nil {
		t.Error("the function is visible in an unrelated evaluation")
	}
}

// TestContextRegisterFunctionShadowsGlobal verifies that a context function
// takes precedence over a global one of the same name without disturbing it.
func TestContextRegisterFunctionShadowsGlobal(t *testing.T) {
	np, err := NewParser(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	np.Ctx.RegisterFunction(&Function{
		Name:      "true",
		Namespace: nsFN,
		F: func(*Context, []Sequence) (Sequence, error) {
			return Sequence{"shadowed"}, nil
		},
	})
	seq, err := np.Evaluate(`true()`)
	if err != nil {
		t.Fatal(err)
	}
	if got := seq.Stringvalue(); got != "shadowed" {
		t.Errorf("true() = %q, want %q", got, "shadowed")
	}

	other, err := NewParser(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	seq, err = other.Evaluate(`true()`)
	if err != nil {
		t.Fatal(err)
	}
	if got := seq.Stringvalue(); got != "true" {
		t.Errorf("the global true() was replaced: got %q, want %q", got, "true")
	}
}
