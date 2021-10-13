package xpath

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

type tokenType int

type token struct {
	Value interface{}
	Typ   tokenType
}

type tokenlist struct {
	pos  int
	toks []token
}

func (tl *tokenlist) peek() (*token, error) {
	return &tl.toks[tl.pos+1], nil
}

func (tl *tokenlist) next() (*token, error) {
	tl.pos++
	return &tl.toks[tl.pos-1], nil
}

func getNum(sr *strings.Reader) float64 {
	var f float64
	fmt.Fscanf(sr, "%f", &f)
	return f
}

func getComment(sr *strings.Reader) (string, error) {
	lvl := 1
	var this, next rune
	var err error
	var comment []rune
	for {
		if this, _, err = sr.ReadRune(); err != nil {
			if err == io.EOF {
				// todo error handling
			}
			return "", err
		}
		if next, _, err = sr.ReadRune(); err != nil {
			if err == io.EOF {
				// todo error handling
			}
			return "", err
		}

		if this == ':' && next == ')' {
			lvl--
			if lvl == 0 {
				break
			}
		}

		if this == '(' && next == ':' {
			lvl++
		}

		comment = append(comment, this)
		if next == ':' || next == '(' {
			sr.UnreadRune()
		} else {
			comment = append(comment, next)
		}

	}

	return string(comment), nil
}

func stringToTokenlist(str string) (*tokenlist, error) {
	var tokens []token
	sr := strings.NewReader(str)
	for {
		r, _, err := sr.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		if '0' <= r && r <= '9' || r == '.' {
			sr.UnreadRune()
			tokens = append(tokens, token{getNum(sr), 0})
		} else if r == '+' {
			tokens = append(tokens, token{r, 0})
		} else if unicode.IsSpace(r) {
			// ignore whitespace
		} else if r == '(' {
			nextRune, _, err := sr.ReadRune()
			if err == io.EOF {
				// what should we do?
				return nil, fmt.Errorf("parse error, unbalanced ( at end")
			}
			if err != nil {
				return nil, err
			}
			if nextRune == ':' {
				cmt, err := getComment(sr)
				if err != nil {
					return nil, err
				}
				fmt.Println(cmt)
			}
		} else {
			fmt.Printf("%q\n", string(r))
		}
	}
	// tokens := []token{{Value: 1, Typ: 0}, {Value: "+", Typ: 0}, {Value: 4, Typ: 0}}
	tl := tokenlist{pos: 0, toks: tokens}
	return &tl, nil
}
