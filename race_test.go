package goxpath

import (
	"strconv"
	"strings"
	"sync"
	"testing"
)

// TestConcurrentEvaluate verifies that the expression cache does not cause data
// races when multiple goroutines evaluate the same XPath expressions on
// different parsers concurrently.
func TestConcurrentEvaluate(t *testing.T) {
	const goroutines = 8
	exprs := []string{
		`/root/sub`,
		`/root/sub[@foo='bar']`,
		`/root/sub[2]`,
		`//sub`,
		`count(/root/sub)`,
		`string(/root/sub[1])`,
		`for $i in /root/sub return string($i/@foo)`,
		// Predicates on a PrimaryExpr go through parseFilterExpr, which is a
		// separate code path from the predicates on axis steps above.
		`(//sub)[1]`,
		`(/root/sub)[@foo='bar']`,
		`(1 to 10)[. mod 2 = 0]`,
	}

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			np, err := NewParser(strings.NewReader(doc))
			if err != nil {
				t.Error(err)
				return
			}
			for i := 0; i < 100; i++ {
				for _, expr := range exprs {
					_, err := np.Evaluate(expr)
					if err != nil {
						t.Error(err)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
}

// TestConcurrentContextFunctions verifies that registering functions per context
// keeps parallel evaluations apart. Registering the same name globally instead
// crashes the process with "concurrent map read and map write", because the
// global registry is an unsynchronized map.
func TestConcurrentContextFunctions(t *testing.T) {
	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			np, err := NewParser(strings.NewReader(doc))
			if err != nil {
				t.Error(err)
				return
			}
			want := strconv.Itoa(id)
			np.Ctx.RegisterFunction(&Function{
				Name:      "goroutine-id",
				Namespace: nsFN,
				F: func(*Context, []Sequence) (Sequence, error) {
					return Sequence{want}, nil
				},
			})
			for i := 0; i < 200; i++ {
				seq, err := np.Evaluate(`goroutine-id()`)
				if err != nil {
					t.Error(err)
					return
				}
				if got := seq.Stringvalue(); got != want {
					t.Errorf("goroutine %d saw %q", id, got)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}
