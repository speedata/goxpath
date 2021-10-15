package xpath

import (
	"fmt"
	"strings"
)

var debuglevel int

func enterStep(tl *tokenlist, step string) {
	fmt.Println(strings.Repeat("  ", debuglevel), ">>", step)
	debuglevel++
}

func leaveStep(tl *tokenlist, step string) {
	debuglevel--
	fmt.Println(strings.Repeat("  ", debuglevel), "<<", step)
}
