package goxpath

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

type tokenType int

const (
	// tokAny contains any token.
	tokAny tokenType = iota
	// tokString contains any characters including whitespace.
	tokString
	// tokVarname represents a variable name.
	tokVarname
	// tokNumber represents a float64.
	tokNumber
	// tokOperator contains a single or double letter operator or path
	// separator.
	tokOperator
	// tokOpenParen is an opening parenthesis (.
	tokOpenParen
	// tokCloseParen is a closing parenthesis ).
	tokCloseParen
	// tokOpenBracket is an opening bracket [.
	tokOpenBracket
	// tokCloseBracket is a closing bracket ].
	tokCloseBracket
	// tokQName is a QName (which might contain one colon).
	tokQName
	// tokComma represents a comma
	tokComma
	// tokDoubleColon represents a word with two colons a the end (axis for
	// example)
	tokDoubleColon
)

func (tt tokenType) String() string {
	switch tt {
	case tokAny:
		return "Any token"
	case tokString:
		return "string"
	case tokVarname:
		return "variable name"
	case tokNumber:
		return "number"
	case tokOperator:
		return "operator"
	case tokOpenParen:
		return "open paren"
	case tokCloseParen:
		return "close paren"
	case tokOpenBracket:
		return "open bracket"
	case tokCloseBracket:
		return "close bracket"
	case tokQName:
		return "QName"
	case tokComma:
		return "comma"
	}
	return "--"
}

type token struct {
	Value interface{}
	Typ   tokenType
}

func (tok *token) isNCName() bool {
	if tok.Typ != tokQName {
		return false
	}
	tokAsString := tok.Value.(string)
	return !strings.ContainsRune(tokAsString, ':')
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
	case tokVarname:
		return "$" + tok.Value.(string)
	case tokOpenParen:
		return "("
	case tokCloseParen:
		return ")"
	case tokOpenBracket:
		return "["
	case tokCloseBracket:
		return "]"
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

// Tokenlist represents units of XPath language elements.
type Tokenlist struct {
	pos           int
	toks          tokens
	attributeMode bool // for Name Test
}

func (tl *Tokenlist) nexttokIsTyp(typ tokenType) bool {
	tok, err := tl.peek()
	if err != nil {
		return false
	}
	return tok.Typ == typ
}

func (tl *Tokenlist) readIfTokenFollow(toks []token) bool {
	pos := tl.pos
	savepos := tl.pos
	for _, tok := range toks {
		peekTok, err := tl.peek()
		if err != nil {
			tl.pos = savepos
			return false
		}
		if peekTok.Value == tok.Value && peekTok.Typ == tok.Typ {
			pos++
		} else {
			tl.pos = savepos
			return false
		}
		tl.read()
	}
	return true
}

// nexttokIsValue looks at the next token and returns true if the value matches.
// Does not move the pointer forward.
func (tl *Tokenlist) nexttokIsValue(val string) bool {
	tok, err := tl.peek()
	if err != nil {
		return false
	}
	if str, ok := tok.Value.(string); ok {
		return str == val
	}
	return false
}

func (tl *Tokenlist) readNexttokIfIsOneOfValue(val []string) (string, bool) {
	tok, err := tl.peek()
	if err != nil {
		return "", false
	}
	if str, ok := tok.Value.(string); ok {
		for _, v := range val {
			if str == v {
				tl.read()
				return v, true
			}
		}
	}
	return "", false
}

func (tl *Tokenlist) readNexttokIfIsOneOfValueAndType(val []string, typ tokenType) (string, bool) {
	tok, err := tl.peek()
	if err != nil {
		return "", false
	}
	if tok.Typ != typ {
		return fmt.Sprint(tok.Value), false
	}
	if str, ok := tok.Value.(string); ok {
		for _, v := range val {
			if str == v {
				tl.read()
				return v, true
			}
		}
	}
	return "", false
}

func (tl *Tokenlist) peek() (*token, error) {
	if len(tl.toks) == tl.pos {
		return nil, io.EOF
	}
	return &tl.toks[tl.pos], nil
}

func (tl *Tokenlist) read() (*token, error) {
	if len(tl.toks) == tl.pos {
		return nil, io.EOF
	}
	tl.pos++
	return &tl.toks[tl.pos-1], nil
}

func (tl *Tokenlist) unread() error {
	tl.pos--
	return nil
}

// skipType reads the next token and returns an error if the token type does not
// match.
func (tl *Tokenlist) skipType(typ tokenType) error {
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

func (tl *Tokenlist) skipNCName(name string) error {
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
				sr.UnreadRune()
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

func stringToTokenlist(str string) (*Tokenlist, error) {
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
		if '0' <= r && r <= '9' {
			sr.UnreadRune()
			tokens = append(tokens, token{getNum(sr), tokNumber})
		} else if r == '.' {
			nextRune, _, err := sr.ReadRune()
			if err == io.EOF {
				tokens = append(tokens, token{".", tokOperator})
				break
			}
			if err != nil {
				return nil, err
			}
			if '0' <= nextRune && nextRune <= '9' {
				sr.UnreadRune()
				sr.UnreadRune()
				tokens = append(tokens, token{getNum(sr), tokNumber})
			} else if nextRune == '.' {
				tokens = append(tokens, token{"..", tokOperator})
			} else {
				sr.UnreadRune()
				tokens = append(tokens, token{".", tokOperator})
			}
		} else if r == '+' || r == '-' || r == '*' || r == '?' || r == '@' || r == '|' || r == '=' {
			tokens = append(tokens, token{string(r), tokOperator})
		} else if r == ',' {
			tokens = append(tokens, token{string(r), tokComma})
		} else if r == '>' || r == '<' {
			nextRune, _, err := sr.ReadRune()
			if err != nil {
				return nil, err
			}
			if nextRune == '=' || nextRune == r {
				tokens = append(tokens, token{string(r) + string(nextRune), tokOperator})
			} else {
				tokens = append(tokens, token{string(r), tokOperator})
				sr.UnreadRune()
			}
		} else if r == '!' {
			nextRune, _, err := sr.ReadRune()
			if err != nil {
				return nil, err
			}
			if nextRune == '=' {
				tokens = append(tokens, token{"!=", tokOperator})
			} else {
				return nil, fmt.Errorf("= expected after !, got %s", string(nextRune))
			}
		} else if r == '/' || r == ':' {
			nextRune, _, err := sr.ReadRune()
			if err == io.EOF {
				tokens = append(tokens, token{string(r), tokOperator})
				break
			}
			if err != nil {
				return nil, err
			}
			if nextRune == r {
				tokens = append(tokens, token{string(r) + string(r), tokOperator})
			} else {
				tokens = append(tokens, token{string(r), tokOperator})
				sr.UnreadRune()
			}
		} else if r == '[' {
			tokens = append(tokens, token{r, tokOpenBracket})
		} else if r == ']' {
			tokens = append(tokens, token{r, tokCloseBracket})
		} else if r == '$' {
			qname, err := getQName(sr)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token{qname, tokVarname})
		} else if unicode.IsSpace(r) {
			// ignore whitespace
		} else if unicode.IsLetter(r) {
			sr.UnreadRune()
			word, err := getQName(sr)
			if err != nil {
				return nil, err
			}
			nextRune, _, err := sr.ReadRune()
			if err == io.EOF {
				tokens = append(tokens, token{word, tokQName})
				break
			}
			if nextRune == ':' {
				tokens = append(tokens, token{strings.TrimSuffix(word, ":"), tokDoubleColon})
			} else {
				sr.UnreadRune()
				tokens = append(tokens, token{word, tokQName})
			}

		} else if r == '\'' || r == '"' {
			sr.UnreadRune()
			str, err := getDelimitedString(sr)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token{str, tokString})
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
				tokens = append(tokens, token{r, tokOpenParen})
			}
		} else if r == ')' {
			tokens = append(tokens, token{r, tokCloseParen})
		} else {
			return nil, fmt.Errorf("Invalid char for xpath expression %q", string(r))
		}
	}
	tl := Tokenlist{pos: 0, toks: tokens}
	return &tl, nil
}
