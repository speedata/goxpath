package goxpath

import "fmt"

// XPathError represents a structured XPath error with an error code,
// description, and optional error value. This implements the error interface
// and is used by try/catch expressions to match error codes.
type XPathError struct {
	Code        string   // e.g. "XPTY0004", "FORG0001", "FOER0000"
	Description string   // human-readable description
	Value       Sequence // optional error value (for fn:error)
}

// Error implements the error interface.
func (e *XPathError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Description)
	}
	return e.Code
}

// NewXPathError creates a new XPathError with the given code and description.
func NewXPathError(code, description string) *XPathError {
	return &XPathError{Code: code, Description: description}
}

// XPathErrorCode extracts the error code from an error, if it is an XPathError.
// Returns the code and true if successful, or empty string and false otherwise.
func XPathErrorCode(err error) (string, bool) {
	var xe *XPathError
	if err == nil {
		return "", false
	}
	if ok := errorAs(err, &xe); ok {
		return xe.Code, true
	}
	// Fallback: try to extract error code from error message string
	msg := err.Error()
	if len(msg) >= 8 {
		// Check for pattern like "XPTY0004" at the start
		prefix := msg[:8]
		if isErrorCode(prefix) {
			return prefix, true
		}
	}
	return "", false
}

// errorAs is a wrapper for errors.As to keep the import clean.
func errorAs(err error, target any) bool {
	// Use type assertion since we know the target type
	if xe, ok := err.(*XPathError); ok {
		if t, ok2 := target.(**XPathError); ok2 {
			*t = xe
			return true
		}
	}
	return false
}

// TypeIDOf returns the XPath type identifier for an item.
func TypeIDOf(itm any) string {
	switch itm := itm.(type) {
	case XSDouble:
		return "xs:double"
	case XSFloat:
		return "xs:float"
	case XSDecimal:
		return "xs:decimal"
	case int:
		return "xs:integer"
	case XSInteger:
		return intSubtypeName[itm.Subtype]
	case float64:
		return "xs:double"
	case string:
		return "xs:string"
	case XSString:
		return strSubtypeName[itm.Subtype]
	case bool:
		return "xs:boolean"
	case XSAnyURI:
		return "xs:anyURI"
	case XSUntypedAtomic:
		return "xs:untypedAtomic"
	case XSHexBinary:
		return "xs:hexBinary"
	case XSBase64Binary:
		return "xs:base64Binary"
	case XSDateTime:
		return "xs:dateTime"
	case XSDate:
		return "xs:date"
	case XSTime:
		return "xs:time"
	case XSDuration:
		return "xs:duration"
	case XSGYear:
		return "xs:gYear"
	case XSGMonth:
		return "xs:gMonth"
	case XSGDay:
		return "xs:gDay"
	case XSGYearMonth:
		return "xs:gYearMonth"
	case XSGMonthDay:
		return "xs:gMonthDay"
	case XSQName:
		return "xs:QName"
	}
	return "unknown"
}

// castAllowed checks if casting from sourceType to targetType is permitted.
// Returns true if the cast is allowed (though it might still fail with FORG0001
// if the value is invalid for the target type).
func castAllowed(sourceType, targetType string) bool {
	// String-derived types can cast to each other and to string/untypedAtomic
	stringTypes := map[string]bool{
		"xs:string": true, "xs:normalizedString": true, "xs:token": true,
		"xs:language": true, "xs:NMTOKEN": true, "xs:Name": true,
		"xs:NCName": true, "xs:ID": true, "xs:IDREF": true, "xs:ENTITY": true,
		"xs:untypedAtomic": true,
	}
	if stringTypes[sourceType] {
		// String types can be cast to anything
		return true
	}
	// Anything can be cast to string types
	if stringTypes[targetType] {
		return true
	}

	// Numeric types can cast to each other and to xs:boolean
	numericTypes := map[string]bool{
		"xs:double": true, "xs:float": true, "xs:decimal": true, "xs:integer": true,
		"xs:long": true, "xs:int": true, "xs:short": true, "xs:byte": true,
		"xs:unsignedLong": true, "xs:unsignedInt": true, "xs:unsignedShort": true, "xs:unsignedByte": true,
		"xs:nonPositiveInteger": true, "xs:nonNegativeInteger": true,
		"xs:negativeInteger": true, "xs:positiveInteger": true,
	}
	if numericTypes[sourceType] && numericTypes[targetType] {
		return true
	}
	if numericTypes[sourceType] && targetType == "xs:boolean" {
		return true
	}
	if sourceType == "xs:boolean" && numericTypes[targetType] {
		return true
	}
	if sourceType == "xs:boolean" && targetType == "xs:boolean" {
		return true
	}

	// Date/time types — strict casting matrix per XPath spec
	// Key: source → allowed targets (in addition to string/untypedAtomic)
	dateTimeCasts := map[string]map[string]bool{
		"xs:dateTime": {
			"xs:dateTime": true, "xs:date": true, "xs:time": true,
			"xs:gYear": true, "xs:gMonth": true, "xs:gDay": true,
			"xs:gYearMonth": true, "xs:gMonthDay": true,
		},
		"xs:date": {
			"xs:dateTime": true, "xs:date": true,
			"xs:gYear": true, "xs:gMonth": true, "xs:gDay": true,
			"xs:gYearMonth": true, "xs:gMonthDay": true,
		},
		"xs:time":       {"xs:time": true},
		"xs:gYear":      {"xs:gYear": true},
		"xs:gMonth":     {"xs:gMonth": true},
		"xs:gDay":       {"xs:gDay": true},
		"xs:gYearMonth": {"xs:gYearMonth": true},
		"xs:gMonthDay":  {"xs:gMonthDay": true},
	}
	if allowed, ok := dateTimeCasts[sourceType]; ok {
		if _, ok := dateTimeCasts[targetType]; ok {
			return allowed[targetType]
		}
	}

	// Duration types can cast to each other
	durationTypes := map[string]bool{
		"xs:duration": true, "xs:dayTimeDuration": true, "xs:yearMonthDuration": true,
	}
	if durationTypes[sourceType] && durationTypes[targetType] {
		return true
	}

	// hexBinary and base64Binary can cast to each other
	if (sourceType == "xs:hexBinary" || sourceType == "xs:base64Binary") &&
		(targetType == "xs:hexBinary" || targetType == "xs:base64Binary") {
		return true
	}

	// xs:anyURI can cast to xs:string (already covered above)
	// xs:QName cannot be cast from most types
	if targetType == "xs:QName" && sourceType != "xs:QName" {
		return false
	}

	// Numeric to date/time is not allowed
	if numericTypes[sourceType] && dateTimeCasts[targetType] != nil {
		return false
	}
	if dateTimeCasts[sourceType] != nil && numericTypes[targetType] {
		return false
	}

	// Boolean to date/time is not allowed
	if sourceType == "xs:boolean" && dateTimeCasts[targetType] != nil {
		return false
	}
	if dateTimeCasts[sourceType] != nil && targetType == "xs:boolean" {
		return false
	}

	// Duration to numeric/boolean is not allowed
	if durationTypes[sourceType] && (numericTypes[targetType] || targetType == "xs:boolean") {
		return false
	}
	if (numericTypes[sourceType] || sourceType == "xs:boolean") && durationTypes[targetType] {
		return false
	}

	// hexBinary/base64Binary to numeric/boolean/date is not allowed
	binaryTypes := map[string]bool{"xs:hexBinary": true, "xs:base64Binary": true}
	if binaryTypes[sourceType] && !binaryTypes[targetType] {
		return false
	}
	if !binaryTypes[sourceType] && binaryTypes[targetType] {
		if sourceType != "xs:string" && sourceType != "xs:untypedAtomic" {
			return false
		}
	}

	// Default: not allowed
	return false
}

// isErrorCode checks if a string looks like an XPath error code (4 letters + 4 digits).
func isErrorCode(s string) bool {
	if len(s) != 8 {
		return false
	}
	for i := 0; i < 4; i++ {
		if s[i] < 'A' || s[i] > 'Z' {
			return false
		}
	}
	for i := 4; i < 8; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
