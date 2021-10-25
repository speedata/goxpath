package xpath

import (
	"fmt"
	"strings"
)

const (
	indent = " "
)

var (
	debugIndentLevel int
	doDebug          bool
)

func init() {
	doDebug = false
}

func enterStep(tl *tokenlist, step string) {
	if doDebug {
		peek, _ := tl.peek()
		fmt.Println(strings.Repeat(indent, debugIndentLevel), ">>", step, peek)
		debugIndentLevel++
	}
}

func leaveStep(tl *tokenlist, step string) {
	if doDebug {
		peek, _ := tl.peek()
		debugIndentLevel--
		fmt.Println(strings.Repeat(indent, debugIndentLevel), "<<", step, peek)
	}
}
