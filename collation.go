package goxpath

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// Collation is a named string comparison rule used by XPath/XSLT functions
// such as fn:compare, fn:contains, xsl:sort etc.
//
// All methods must be safe for concurrent use.
type Collation interface {
	// URI returns the canonical URI identifying this collation.
	URI() string
	// Compare returns -1, 0 or +1 like strings.Compare under this collation.
	Compare(a, b string) int
	// Equal reports whether the two strings are equal under this collation.
	Equal(a, b string) bool
	// Contains reports whether sub occurs as a (collation-equal) substring of s.
	Contains(s, sub string) bool
	// StartsWith reports whether s begins with prefix under this collation.
	StartsWith(s, prefix string) bool
	// EndsWith reports whether s ends with suffix under this collation.
	EndsWith(s, suffix string) bool
	// SubstringBefore returns the part of s before the first occurrence of sub,
	// or "" if sub does not occur in s. The empty sub returns "".
	SubstringBefore(s, sub string) string
	// SubstringAfter returns the part of s after the first occurrence of sub,
	// or "" if sub does not occur in s. The empty sub returns s.
	SubstringAfter(s, sub string) string
	// Key returns an opaque sort key suitable for use as a hash-map key.
	// Two strings are Equal iff their Keys are equal.
	Key(s string) string
}

// Standard collation URIs defined by XPath 3.1 / XSLT 3.0.
const (
	CodepointCollationURI            = "http://www.w3.org/2005/xpath-functions/collation/codepoint"
	HTMLAsciiCaseInsensitiveURI      = "http://www.w3.org/2005/xpath-functions/collation/html-ascii-case-insensitive"
	UCACollationURI                  = "http://www.w3.org/2013/collation/UCA"
	UCACollationFallbackURI          = "http://www.w3.org/2013/collation/UCA/" // tolerated alias
)

// CollationFactory builds a Collation from URI parameters.
// The full URI is passed for round-tripping; params is the parsed query string.
type CollationFactory func(uri string, params url.Values) (Collation, error)

var (
	collationMu       sync.RWMutex
	collationFactory  = map[string]CollationFactory{}
	collationInstance = map[string]Collation{} // memoized resolved collations
)

// RegisterCollation registers a collation factory for a given URI prefix.
// The most specific (longest) match wins. Pass an exact URI for parameter-less
// collations such as the codepoint collation.
func RegisterCollation(uri string, factory CollationFactory) {
	collationMu.Lock()
	defer collationMu.Unlock()
	collationFactory[uri] = factory
}

// ResolveCollation looks up a collation by URI, instantiating it on demand.
// Returns FOCH0002 if the URI cannot be resolved.
func ResolveCollation(uri string) (Collation, error) {
	collationMu.RLock()
	if c, ok := collationInstance[uri]; ok {
		collationMu.RUnlock()
		return c, nil
	}
	// Find the longest registered prefix that matches.
	var bestKey string
	var bestFactory CollationFactory
	for k, f := range collationFactory {
		if k == uri || (strings.HasSuffix(k, "?") && strings.HasPrefix(uri, k)) ||
			(!strings.Contains(k, "?") && (uri == k || strings.HasPrefix(uri, k+"?"))) {
			if len(k) > len(bestKey) {
				bestKey = k
				bestFactory = f
			}
		}
	}
	collationMu.RUnlock()
	if bestFactory == nil {
		return nil, NewXPathError("FOCH0002", fmt.Sprintf("unknown collation %q", uri))
	}
	// Parse query parameters if any.
	var params url.Values
	if i := strings.Index(uri, "?"); i >= 0 {
		q, err := url.ParseQuery(strings.ReplaceAll(uri[i+1:], ";", "&"))
		if err != nil {
			return nil, NewXPathError("FOCH0002", fmt.Sprintf("invalid collation parameters in %q: %v", uri, err))
		}
		params = q
	} else {
		params = url.Values{}
	}
	c, err := bestFactory(uri, params)
	if err != nil {
		return nil, err
	}
	collationMu.Lock()
	collationInstance[uri] = c
	collationMu.Unlock()
	return c, nil
}

// CodepointCollation returns the (cached) Unicode codepoint collation.
func CodepointCollation() Collation {
	c, _ := ResolveCollation(CodepointCollationURI)
	return c
}

// ---------------------------------------------------------------------------
// Codepoint collation: simple Unicode codepoint comparison.
// ---------------------------------------------------------------------------

type codepointCollation struct{}

func (codepointCollation) URI() string                  { return CodepointCollationURI }
func (codepointCollation) Compare(a, b string) int      { return strings.Compare(a, b) }
func (codepointCollation) Equal(a, b string) bool       { return a == b }
func (codepointCollation) Contains(s, sub string) bool  { return strings.Contains(s, sub) }
func (codepointCollation) StartsWith(s, p string) bool  { return strings.HasPrefix(s, p) }
func (codepointCollation) EndsWith(s, suf string) bool  { return strings.HasSuffix(s, suf) }
func (codepointCollation) Key(s string) string          { return s }
func (c codepointCollation) SubstringBefore(s, sub string) string {
	if sub == "" {
		return ""
	}
	if i := strings.Index(s, sub); i >= 0 {
		return s[:i]
	}
	return ""
}
func (c codepointCollation) SubstringAfter(s, sub string) string {
	if sub == "" {
		return s
	}
	if i := strings.Index(s, sub); i >= 0 {
		return s[i+len(sub):]
	}
	return ""
}

// ---------------------------------------------------------------------------
// HTML ASCII case-insensitive collation.
// Defined in F&O 3.1: ASCII letters are folded to lower case, everything else
// compared by Unicode codepoint.
// ---------------------------------------------------------------------------

type htmlAsciiCICollation struct{}

func asciiFold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b.WriteByte(c)
	}
	return b.String()
}

func (htmlAsciiCICollation) URI() string             { return HTMLAsciiCaseInsensitiveURI }
func (htmlAsciiCICollation) Compare(a, b string) int { return strings.Compare(asciiFold(a), asciiFold(b)) }
func (htmlAsciiCICollation) Equal(a, b string) bool  { return asciiFold(a) == asciiFold(b) }
func (htmlAsciiCICollation) Contains(s, sub string) bool {
	return strings.Contains(asciiFold(s), asciiFold(sub))
}
func (htmlAsciiCICollation) StartsWith(s, p string) bool {
	return strings.HasPrefix(asciiFold(s), asciiFold(p))
}
func (htmlAsciiCICollation) EndsWith(s, suf string) bool {
	return strings.HasSuffix(asciiFold(s), asciiFold(suf))
}
func (htmlAsciiCICollation) Key(s string) string { return asciiFold(s) }
func (htmlAsciiCICollation) SubstringBefore(s, sub string) string {
	if sub == "" {
		return ""
	}
	if i := strings.Index(asciiFold(s), asciiFold(sub)); i >= 0 {
		return s[:i]
	}
	return ""
}
func (htmlAsciiCICollation) SubstringAfter(s, sub string) string {
	if sub == "" {
		return s
	}
	if i := strings.Index(asciiFold(s), asciiFold(sub)); i >= 0 {
		return s[i+len(sub):]
	}
	return ""
}

// ---------------------------------------------------------------------------
// UCA collation backed by golang.org/x/text/collate.
// ---------------------------------------------------------------------------

type ucaCollation struct {
	uri       string
	collator  *collate.Collator
	primary   bool // strength=primary -> contains/substring fallbacks
	tag       language.Tag
	caseFirst string // "upper"|"lower"|""
}

// recognizedUCAParams lists the query parameters defined by the F&O 3.1
// UCA collation URI scheme. Anything else triggers FOCH0002 when fallback=no.
var recognizedUCAParams = map[string]bool{
	"fallback":     true,
	"lang":         true,
	"version":     true,
	"strength":     true,
	"maxVariable":  true,
	"alternate":    true,
	"backwards":    true,
	"normalization": true,
	"caseLevel":    true,
	"caseFirst":    true,
	"numeric":      true,
	"reorder":      true,
}

func newUCACollation(uri string, params url.Values) (Collation, error) {
	// We map UCA parameters onto BCP-47 Unicode extensions and on the
	// limited set of options exposed by golang.org/x/text/collate.
	noFallback := strings.EqualFold(params.Get("fallback"), "no")

	// Reject unrecognized parameters when fallback=no.
	if noFallback {
		for k := range params {
			if !recognizedUCAParams[k] {
				return nil, NewXPathError("FOCH0002", fmt.Sprintf("unsupported UCA parameter %q", k))
			}
		}
		// We do not (yet) support these tuning knobs natively.
		for _, k := range []string{
			"maxVariable", "reorder", "version", "normalization",
			"backwards", "alternate", "caseFirst", "caseLevel",
		} {
			if params.Get(k) != "" {
				return nil, NewXPathError("FOCH0002", fmt.Sprintf("unsupported UCA parameter %q", k))
			}
		}
	}

	base := language.Und
	if l := params.Get("lang"); l != "" {
		t, err := language.Parse(l)
		if err != nil {
			if noFallback {
				return nil, NewXPathError("FOCH0002", fmt.Sprintf("unsupported language %q", l))
			}
		} else {
			base = t
		}
	}

	// Build BCP-47 -u- extension keys for collation tuning.
	var ext []string
	strength := strings.ToLower(params.Get("strength"))
	switch strength {
	case "", "tertiary", "3":
		// default
	case "primary", "1":
		ext = append(ext, "ks", "level1")
	case "secondary", "2":
		ext = append(ext, "ks", "level2")
	case "quaternary", "4":
		ext = append(ext, "ks", "level4")
	case "identical", "5":
		ext = append(ext, "ks", "identic")
	default:
		if noFallback {
			return nil, NewXPathError("FOCH0002", fmt.Sprintf("invalid UCA strength %q", strength))
		}
		strength = ""
	}
	caseFirst := strings.ToLower(params.Get("caseFirst"))
	switch caseFirst {
	case "":
	case "upper":
		ext = append(ext, "kf", "upper")
	case "lower":
		ext = append(ext, "kf", "lower")
	default:
		if noFallback {
			return nil, NewXPathError("FOCH0002", fmt.Sprintf("invalid caseFirst %q", caseFirst))
		}
		caseFirst = ""
	}
	if cl := strings.ToLower(params.Get("caseLevel")); cl == "yes" || cl == "true" {
		ext = append(ext, "kc", "true")
	}
	if alt := strings.ToLower(params.Get("alternate")); alt == "shifted" {
		ext = append(ext, "ka", "shifted")
	}

	tag := base
	if len(ext) > 0 {
		var sb strings.Builder
		sb.WriteString(base.String())
		if !strings.Contains(base.String(), "-u-") {
			sb.WriteString("-u")
		}
		for i := 0; i < len(ext); i += 2 {
			sb.WriteByte('-')
			sb.WriteString(ext[i])
			sb.WriteByte('-')
			sb.WriteString(ext[i+1])
		}
		if t, err := language.Parse(sb.String()); err == nil {
			tag = t
		}
	}

	var opts []collate.Option
	opts = append(opts, collate.OptionsFromTag(tag))
	if numeric := strings.ToLower(params.Get("numeric")); numeric == "yes" || numeric == "true" {
		opts = append(opts, collate.Numeric)
	}

	c := collate.New(tag, opts...)
	return &ucaCollation{
		uri:       uri,
		collator:  c,
		primary:   strength == "primary" || strength == "1",
		tag:       tag,
		caseFirst: caseFirst,
	}, nil
}

func (u *ucaCollation) URI() string { return u.uri }

func (u *ucaCollation) Compare(a, b string) int {
	return u.collator.CompareString(a, b)
}

func (u *ucaCollation) Equal(a, b string) bool {
	return u.collator.CompareString(a, b) == 0
}

// Contains/StartsWith/EndsWith/SubstringBefore/SubstringAfter for UCA
// fall back to a rune-by-rune scan using Compare on equal-length windows.
// This is O(n*m) but correct for collation-aware substring matching.
func (u *ucaCollation) windowMatch(s, sub string) (start, end int, ok bool) {
	if sub == "" {
		return 0, 0, true
	}
	subRunes := []rune(sub)
	sRunes := []rune(s)
	if len(subRunes) > len(sRunes) {
		return 0, 0, false
	}
	for i := 0; i+len(subRunes) <= len(sRunes); i++ {
		if u.Equal(string(sRunes[i:i+len(subRunes)]), sub) {
			// Convert rune indices to byte offsets.
			start = len(string(sRunes[:i]))
			end = start + len(string(sRunes[i:i+len(subRunes)]))
			return start, end, true
		}
	}
	return 0, 0, false
}

func (u *ucaCollation) Contains(s, sub string) bool {
	_, _, ok := u.windowMatch(s, sub)
	return ok
}

func (u *ucaCollation) StartsWith(s, prefix string) bool {
	if prefix == "" {
		return true
	}
	pRunes := []rune(prefix)
	sRunes := []rune(s)
	if len(pRunes) > len(sRunes) {
		return false
	}
	return u.Equal(string(sRunes[:len(pRunes)]), prefix)
}

func (u *ucaCollation) EndsWith(s, suffix string) bool {
	if suffix == "" {
		return true
	}
	sufRunes := []rune(suffix)
	sRunes := []rune(s)
	if len(sufRunes) > len(sRunes) {
		return false
	}
	return u.Equal(string(sRunes[len(sRunes)-len(sufRunes):]), suffix)
}

func (u *ucaCollation) SubstringBefore(s, sub string) string {
	if sub == "" {
		return ""
	}
	start, _, ok := u.windowMatch(s, sub)
	if !ok {
		return ""
	}
	return s[:start]
}

func (u *ucaCollation) SubstringAfter(s, sub string) string {
	if sub == "" {
		return s
	}
	_, end, ok := u.windowMatch(s, sub)
	if !ok {
		return ""
	}
	return s[end:]
}

func (u *ucaCollation) Key(s string) string {
	buf := &collate.Buffer{}
	k := u.collator.KeyFromString(buf, s)
	return string(k)
}

// strconvDummy keeps strconv imported to avoid removal if used by future params.
var _ = strconv.Itoa

// collationFromArg picks the collation referred to by an optional URI argument.
// If the argument sequence is empty or absent, the static default collation is
// returned. Relative URIs are resolved against the static base URI from
// ctx.Store["baseURI"] if present.
func collationFromArg(ctx *Context, arg Sequence) (Collation, error) {
	if len(arg) == 0 {
		return ctx.Collation(), nil
	}
	uri, err := StringValue(arg)
	if err != nil {
		return nil, err
	}
	if uri == "" {
		return ctx.Collation(), nil
	}
	// Resolve relative URI against the static base URI.
	if !strings.Contains(uri, ":") {
		if ctx.Store != nil {
			if base, ok := ctx.Store["baseURI"].(string); ok && base != "" {
				if bu, err := url.Parse(base); err == nil {
					if ru, err := url.Parse(uri); err == nil {
						uri = bu.ResolveReference(ru).String()
					}
				}
			}
		}
	}
	return ResolveCollation(uri)
}

func init() {
	RegisterCollation(CodepointCollationURI, func(uri string, _ url.Values) (Collation, error) {
		return codepointCollation{}, nil
	})
	RegisterCollation(HTMLAsciiCaseInsensitiveURI, func(uri string, _ url.Values) (Collation, error) {
		return htmlAsciiCICollation{}, nil
	})
	RegisterCollation(UCACollationURI, newUCACollation)
}
