package goxpath

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/speedata/goxml"
)

const nsJSONXML = "http://www.w3.org/2005/xpath-functions"

// fnJSONToXML implements fn:json-to-xml($json as xs:string) as document-node()
// and fn:json-to-xml($json as xs:string, $options as map(*)) as document-node().
func fnJSONToXML(ctx *Context, args []Sequence) (Sequence, error) {
	jsonStr, err := StringValue(args[0])
	if err != nil {
		return nil, fmt.Errorf("json-to-xml: %w", err)
	}

	// Parse options (second argument) if present.
	var escapeOpt bool
	if len(args) > 1 && len(args[1]) > 0 {
		if m, ok := args[1][0].(*XPathMap); ok {
			if v, found := m.Get("escape"); found {
				if len(v) > 0 {
					if b, ok := v[0].(bool); ok {
						escapeOpt = b
					}
				}
			}
		}
	}

	doc := &goxml.XMLDocument{ID: goxml.NewID()}
	rootElt, err := jsonValueToElement(jsonStr, escapeOpt)
	if err != nil {
		return nil, fmt.Errorf("json-to-xml: %w", err)
	}
	doc.Append(rootElt)
	return Sequence{doc}, nil
}

// jsonValueToElement parses a JSON string and returns the top-level element.
func jsonValueToElement(jsonStr string, escapeOpt bool) (*goxml.Element, error) {
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" {
		return nil, fmt.Errorf("empty JSON input")
	}

	dec := json.NewDecoder(strings.NewReader(jsonStr))
	dec.UseNumber()

	val, err := jsonDecodeValue(dec)
	if err != nil {
		return nil, err
	}
	return jsonBuildElement(val, "", escapeOpt)
}

// jsonDecodeValue reads one JSON value from the decoder.
func jsonDecodeValue(dec *json.Decoder) (interface{}, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}

	switch v := tok.(type) {
	case json.Delim:
		switch v {
		case '{':
			return jsonDecodeObject(dec)
		case '[':
			return jsonDecodeArray(dec)
		default:
			return nil, fmt.Errorf("unexpected delimiter %v", v)
		}
	case json.Number:
		return v, nil
	case string:
		return v, nil
	case bool:
		return v, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected JSON token %T", tok)
	}
}

type jsonObject struct {
	keys   []string
	values []interface{}
}

func jsonDecodeObject(dec *json.Decoder) (*jsonObject, error) {
	obj := &jsonObject{}
	for dec.More() {
		// Read key.
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("expected string key, got %T", keyTok)
		}
		// Read value.
		val, err := jsonDecodeValue(dec)
		if err != nil {
			return nil, err
		}
		obj.keys = append(obj.keys, key)
		obj.values = append(obj.values, val)
	}
	// Consume closing '}'.
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return obj, nil
}

func jsonDecodeArray(dec *json.Decoder) ([]interface{}, error) {
	var arr []interface{}
	for dec.More() {
		val, err := jsonDecodeValue(dec)
		if err != nil {
			return nil, err
		}
		arr = append(arr, val)
	}
	// Consume closing ']'.
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return arr, nil
}

// jsonBuildElement creates an XML element for a JSON value.
// key is the JSON object key (empty for top-level values).
func jsonBuildElement(val interface{}, key string, escapeOpt bool) (*goxml.Element, error) {
	switch v := val.(type) {
	case *jsonObject:
		return jsonBuildMap(v, key, escapeOpt)
	case []interface{}:
		return jsonBuildArray(v, key, escapeOpt)
	case string:
		return jsonBuildString(v, key, escapeOpt)
	case json.Number:
		return jsonBuildNumber(v, key)
	case bool:
		return jsonBuildBoolean(v, key)
	case nil:
		return jsonBuildNull(key)
	default:
		return nil, fmt.Errorf("unexpected value type %T", val)
	}
}

func jsonNewElement(localName string, key string) *goxml.Element {
	elt := goxml.NewElement()
	elt.ID = goxml.NewID()
	elt.Name = localName
	elt.Prefix = "j"
	elt.Namespaces["j"] = nsJSONXML
	if key != "" {
		elt.Append(goxml.Attribute{
			ID:   goxml.NewID(),
			Name: "key",
			Value: key,
		})
	}
	return elt
}

func jsonBuildMap(obj *jsonObject, key string, escapeOpt bool) (*goxml.Element, error) {
	elt := jsonNewElement("map", key)
	for i, k := range obj.keys {
		child, err := jsonBuildElement(obj.values[i], k, escapeOpt)
		if err != nil {
			return nil, err
		}
		elt.Append(child)
	}
	return elt, nil
}

func jsonBuildArray(arr []interface{}, key string, escapeOpt bool) (*goxml.Element, error) {
	elt := jsonNewElement("array", key)
	for _, item := range arr {
		child, err := jsonBuildElement(item, "", escapeOpt)
		if err != nil {
			return nil, err
		}
		elt.Append(child)
	}
	return elt, nil
}

func jsonBuildString(s string, key string, escapeOpt bool) (*goxml.Element, error) {
	elt := jsonNewElement("string", key)
	if escapeOpt {
		// In escape mode, keep JSON escape sequences and mark with escaped="true".
		elt.Append(goxml.Attribute{ID: goxml.NewID(), Name: "escaped", Value: "true"})
	}
	elt.Append(goxml.CharData{ID: goxml.NewID(), Contents: s})
	return elt, nil
}

func jsonBuildNumber(n json.Number, key string) (*goxml.Element, error) {
	elt := jsonNewElement("number", key)
	// Validate and normalize.
	numStr := n.String()
	if _, err := strconv.ParseFloat(numStr, 64); err != nil {
		return nil, fmt.Errorf("invalid JSON number: %s", numStr)
	}
	elt.Append(goxml.CharData{ID: goxml.NewID(), Contents: numStr})
	return elt, nil
}

func jsonBuildBoolean(b bool, key string) (*goxml.Element, error) {
	elt := jsonNewElement("boolean", key)
	if b {
		elt.Append(goxml.CharData{ID: goxml.NewID(), Contents: "true"})
	} else {
		elt.Append(goxml.CharData{ID: goxml.NewID(), Contents: "false"})
	}
	return elt, nil
}

func jsonBuildNull(key string) (*goxml.Element, error) {
	elt := jsonNewElement("null", key)
	return elt, nil
}

// fnXMLToJSON implements fn:xml-to-json($input as node()) as xs:string
// and fn:xml-to-json($input as node(), $options as map(*)) as xs:string.
func fnXMLToJSON(ctx *Context, args []Sequence) (Sequence, error) {
	if len(args[0]) == 0 {
		return nil, fmt.Errorf("xml-to-json: empty input")
	}

	// Find the root element: handle both XMLDocument and Element.
	var rootElt *goxml.Element
	switch n := args[0][0].(type) {
	case *goxml.XMLDocument:
		for _, child := range n.Children() {
			if elt, ok := child.(*goxml.Element); ok {
				rootElt = elt
				break
			}
		}
		if rootElt == nil {
			return nil, fmt.Errorf("xml-to-json: no element child in document")
		}
	case *goxml.Element:
		rootElt = n
	default:
		return nil, fmt.Errorf("xml-to-json: expected node, got %T", args[0][0])
	}

	var sb strings.Builder
	if err := xmlToJSONSerialize(&sb, rootElt); err != nil {
		return nil, fmt.Errorf("xml-to-json: %w", err)
	}
	return Sequence{sb.String()}, nil
}

// xmlToJSONSerialize recursively serializes an element in the JSON XML
// namespace to JSON.
func xmlToJSONSerialize(sb *strings.Builder, elt *goxml.Element) error {
	switch elt.Name {
	case "map":
		sb.WriteByte('{')
		first := true
		for _, child := range elt.Children() {
			childElt, ok := child.(*goxml.Element)
			if !ok {
				continue
			}
			if !first {
				sb.WriteByte(',')
			}
			first = false
			key := xmlToJSONGetAttr(childElt, "key")
			xmlToJSONWriteString(sb, key)
			sb.WriteByte(':')
			if err := xmlToJSONSerialize(sb, childElt); err != nil {
				return err
			}
		}
		sb.WriteByte('}')
	case "array":
		sb.WriteByte('[')
		first := true
		for _, child := range elt.Children() {
			childElt, ok := child.(*goxml.Element)
			if !ok {
				continue
			}
			if !first {
				sb.WriteByte(',')
			}
			first = false
			if err := xmlToJSONSerialize(sb, childElt); err != nil {
				return err
			}
		}
		sb.WriteByte(']')
	case "string":
		xmlToJSONWriteString(sb, xmlToJSONTextContent(elt))
	case "number":
		sb.WriteString(xmlToJSONTextContent(elt))
	case "boolean":
		sb.WriteString(xmlToJSONTextContent(elt))
	case "null":
		sb.WriteString("null")
	default:
		return fmt.Errorf("unexpected element <%s>", elt.Name)
	}
	return nil
}

// xmlToJSONTextContent returns the concatenated text content of an element.
func xmlToJSONTextContent(elt *goxml.Element) string {
	var sb strings.Builder
	for _, child := range elt.Children() {
		if cd, ok := child.(goxml.CharData); ok {
			sb.WriteString(cd.Contents)
		}
	}
	return sb.String()
}

// xmlToJSONGetAttr returns the value of the named attribute, or "".
func xmlToJSONGetAttr(elt *goxml.Element, name string) string {
	for _, attr := range elt.Attributes() {
		if attr.Name == name {
			return attr.Value
		}
	}
	return ""
}

// xmlToJSONWriteString writes a JSON-escaped string (with surrounding quotes).
func xmlToJSONWriteString(sb *strings.Builder, s string) {
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		case '\b':
			sb.WriteString(`\b`)
		case '\f':
			sb.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(sb, `\u%04x`, r)
			} else {
				sb.WriteRune(r)
			}
		}
	}
	sb.WriteByte('"')
}
