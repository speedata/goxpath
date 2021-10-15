package xpath

import (
	"fmt"
	"strings"
)

var debuglevel int

func enterStep(tl *tokenlist, step string) {
	peek, _ := tl.peek()
	fmt.Println(strings.Repeat("  ", debuglevel), ">>", step, peek)
	debuglevel++
}

func leaveStep(tl *tokenlist, step string) {
	peek, _ := tl.peek()
	debuglevel--
	fmt.Println(strings.Repeat("  ", debuglevel), "<<", step, peek)
}
