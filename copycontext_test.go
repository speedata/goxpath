package goxpath

import (
	"testing"
)

// TestCopyContextPositionsBug tests that CopyContext correctly copies
// ctxPositions and ctxLengths. There is a bug on line 87 where
//
//	ctx.ctxLengths = append(ctx.ctxPositions, l)
//
// should be:
//
//	ctx.ctxPositions = append(ctx.ctxPositions, l)
//
// The bug causes:
//   - ctxPositions to be nil in the copy (never assigned)
//   - ctxLengths to be overwritten with a single-element slice
//     containing only the last position value
func TestCopyContextPositionsBug(t *testing.T) {
	ctx := &Context{
		vars:         make(map[string]Sequence),
		Namespaces:   make(map[string]string),
		sequence:     Sequence{"a", "b", "c"},
		ctxPositions: []int{1, 2, 3},
		ctxLengths:   []int{3, 3, 3},
		Pos:          2,
	}

	copied := CopyContext(ctx)

	// ctxPositions must be copied
	if got := len(copied.ctxPositions); got != 3 {
		t.Errorf("len(copied.ctxPositions) = %d, want 3 — ctxPositions was not copied", got)
	}
	for i, want := range []int{1, 2, 3} {
		if i < len(copied.ctxPositions) {
			if got := copied.ctxPositions[i]; got != want {
				t.Errorf("copied.ctxPositions[%d] = %d, want %d", i, got, want)
			}
		}
	}

	// ctxLengths must be preserved (not overwritten by the second loop)
	if got := len(copied.ctxLengths); got != 3 {
		t.Errorf("len(copied.ctxLengths) = %d, want 3 — ctxLengths was overwritten", got)
	}
	for i, want := range []int{3, 3, 3} {
		if i < len(copied.ctxLengths) {
			if got := copied.ctxLengths[i]; got != want {
				t.Errorf("copied.ctxLengths[%d] = %d, want %d", i, got, want)
			}
		}
	}

	// Original must not be modified
	if got := len(ctx.ctxPositions); got != 3 {
		t.Errorf("original ctxPositions modified: len = %d, want 3", got)
	}
	if got := len(ctx.ctxLengths); got != 3 {
		t.Errorf("original ctxLengths modified: len = %d, want 3", got)
	}
}
