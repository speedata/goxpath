package xpath

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

type tokenType int

const (
	// TokAny contains any token.
	TokAny tokenType = iota
	// TokString contains any characters including whitespace.
	TokString
	// TokVarname represents a variable name.
	TokVarname
	// TokNumber represents a float64.
	TokNumber
	// TokOperator contains a single or double letter operator or path sepearator.
	TokOperator
	// TokOpenParen is an opening parenthesis (.
	TokOpenParen
	// TokCloseParen is a closing parenthesis ).
	TokCloseParen
	// TokOpenBracket is an opeing bracket [.
	TokOpenBracket
	// TokCloseBracket is a closing bracket ].
	TokCloseBracket
	// TokQName is a QName (which might contain one colon).
	TokQName
)

func (tt tokenType) String() string {
	switch tt {
	case TokAny:
		return "Any token"
	case TokString:
		return "string"
	case TokVarname:
		return "variable name"
	case TokNumber:
		return "number"
	case TokOperator:
		return "operator"
	case TokOpenParen:
		return "open paren"
	case TokCloseParen:
		return "close paren"
	case TokOpenBracket:
		return "open bracket"
	case TokCloseBracket:
		return "close bracket"
	case TokQName:
		return "QName"
	}
	return "--"
}

type token struct {
	Value interface{}
	Typ   tokenType
}

func (tok *token) isNCName() bool {
	if tok.Typ != TokQName {
		return false
	}
	return !strings.ContainsRune(tok.Value.(string), ':')
}

type tokens []token

func (toks tokens) String() string {
	var str []string
	for _, t := range toks {
		str = append(str, t.String())
	}
	return strings.Join(str, " ")
}

func (tok token) String() string {
	switch tok.Typ {
	case TokVarname:
		return "$" + tok.Value.(string)
	case TokOpenParen:
		return "("
	case TokCloseParen:
		return ")"
	}

	switch v := tok.Value.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%f", v)
	default:
		fmt.Printf("v %#v\n", v)
		return "***"
	}
}

type tokenlist struct {
	pos  int
	toks tokens
}

func (tl *tokenlist) peek() (*token, error) {
	return &tl.toks[tl.pos+1], nil
}

func (tl *tokenlist) read() (*token, error) {
	if len(tl.toks) == tl.pos {
		return nil, io.EOF
	}
	tl.pos++
	return &tl.toks[tl.pos-1], nil
}

func (tl *tokenlist) unread() error {
	tl.pos--
	return nil
}

func (tl *tokenlist) skipType(typ tokenType) error {
	var val *token
	var err error
	if val, err = tl.read(); err != nil {
		return err
	}
	if val.Typ != typ {
		return fmt.Errorf("expect %s, got %s", typ, val.Typ)
	}
	return nil
}

func (tl *tokenlist) skipNCName(name string) error {
	var val *token
	var err error
	if val, err = tl.read(); err != nil {
		return err
	}
	if !val.isNCName() {
		return fmt.Errorf("expect %s, got %s", name, val.Typ)
	}
	if str, ok := val.Value.(string); ok && str != name {
		return fmt.Errorf("expect %s, got %s", name, str)
	}
	return nil
}

func getNum(sr *strings.Reader) float64 {
	var f float64
	fmt.Fscanf(sr, "%f", &f)
	return f
}

// getQName reads until any non QName rune is found.
func getQName(sr *strings.Reader) (string, error) {
	var word []rune
	var hasColon bool
	for {
		r, _, err := sr.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '·' || r == '‿' || r == '⁀' {
			word = append(word, r)
		} else if r == ':' {
			if hasColon {
				break
			}
			word = append(word, r)
			hasColon = true
		} else {
			sr.UnreadRune()
			break
		}
	}
	return string(word), nil
}

func getDelimitedString(sr *strings.Reader) (string, error) {
	var str []rune
	delim, _, err := sr.ReadRune()
	if err != nil {
		return "", err
	}
	for {
		r, _, err := sr.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if r == delim {
			break
		} else {
			str = append(str, r)
		}

	}
	return string(str), err
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
			tokens = append(tokens, token{getNum(sr), TokNumber})
		} else if r == '+' || r == '-' || r == '*' || r == '?' || r == '@' || r == '|' || r == ',' || r == '=' {
			tokens = append(tokens, token{string(r), TokOperator})
		} else if r == '>' || r == '<' {
			nextRune, _, err := sr.ReadRune()
			if err != nil {
				return nil, err
			}
			if nextRune == '=' || nextRune == r {
				tokens = append(tokens, token{string(r) + string(nextRune), TokOperator})
			} else {
				tokens = append(tokens, token{string(r), TokOperator})
				sr.UnreadRune()
			}
		} else if r == '!' {
			nextRune, _, err := sr.ReadRune()
			if err != nil {
				return nil, err
			}
			if nextRune == '=' {
				tokens = append(tokens, token{"!=", TokOperator})
			} else {
				return nil, fmt.Errorf("= expected after !, got %s", string(nextRune))
			}
		} else if r == '/' || r == ':' {
			nextRune, _, err := sr.ReadRune()
			if err != nil {
				return nil, err
			}
			if nextRune == r {
				tokens = append(tokens, token{string(r) + string(r), TokOperator})
			} else {
				tokens = append(tokens, token{string(r), TokOperator})
				sr.UnreadRune()
			}
		} else if r == '[' {
			tokens = append(tokens, token{r, TokOpenBracket})
		} else if r == ']' {
			tokens = append(tokens, token{r, TokCloseBracket})
		} else if r == '$' {
			qname, err := getQName(sr)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token{qname, TokVarname})
		} else if unicode.IsSpace(r) {
			// ignore whitespace
		} else if unicode.IsLetter(r) {
			sr.UnreadRune()
			word, err := getQName(sr)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token{word, TokQName})
		} else if r == '\'' || r == '"' {
			sr.UnreadRune()
			str, err := getDelimitedString(sr)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token{str, TokString})
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
				_, err := getComment(sr)
				if err != nil {
					return nil, err
				}
				// comments are ignored
				// tokens = append(tokens, token{cmt, TokAny})
			} else {
				sr.UnreadRune()
				tokens = append(tokens, token{r, TokOpenParen})
			}
		} else if r == ')' {
			tokens = append(tokens, token{r, TokCloseParen})
		} else {
			fmt.Printf("%q\n", string(r))
		}
	}
	tl := tokenlist{pos: 0, toks: tokens}
	return &tl, nil
}
