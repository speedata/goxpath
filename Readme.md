[![Go reference documentation](https://img.shields.io/badge/doc-go%20reference-73FA79)](https://pkg.go.dev/github.com/speedata/goxpath)

XPath 2.0 package for Go.


License: BSD-3-Clause License


## Usage

`myfile.xml`:

```xml
<data>
    <p>hello world!</p>
    <p>hello XML!</p>
</data>
```

`main.go`:

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/speedata/goxpath"
)

func dothings() error {
	r, err := os.Open("myfile.xml")
	if err != nil {
		return err
	}
	xp, err := goxpath.NewParser(r)
	if err != nil {
		return err
	}
	seq, err := xp.Evaluate("for $i in /data/p return string($i)")
	if err != nil {
		return err
	}
	for _, itm := range seq {
		fmt.Println(itm)
	}
	return nil
}

func main() {
	if err := dothings(); err != nil {
		log.Fatal(err)
	}
}
```

## Limitations

* No schema types
* No collations
* Not all functions are implemented (see list below)


## Implemented functions

This list is copied from [XQuery 1.0 and XPath 2.0 Functions and Operators (Second Edition)](https://www.w3.org/TR/xquery-operators)

### Accessors

| Function                                                      | Accessor     | Accepts                         | Returns   |
| ------------------------------------------------------------- | ------------ | ------------------------------- | --------- |
| [string](https://www.w3.org/TR/xquery-operators/#func-string) | string-value | an optional item or no argument | xs:string |


### Functions on Numeric Values

| Function                                                        | Meaning                                                                                            |
| --------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| [abs](https://www.w3.org/TR/xquery-operators/#func-abs)         | Returns the absolute value of the argument.                                                        |
| [ceiling](https://www.w3.org/TR/xquery-operators/#func-ceiling) | Returns the smallest number with no fractional part that is greater than or equal to the argument. |
| [floor](https://www.w3.org/TR/xquery-operators/#func-floor)     | Returns the largest number with no fractional part that is less than or equal to the argument.     |
| [round](https://www.w3.org/TR/xquery-operators/#func-round)     | Rounds to the nearest number with no fractional part.                                              |

### Functions to Assemble and Disassemble Strings

| Function                                                                                  | Meaning                                                                   |
| ----------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| [codepoints-to-string](https://www.w3.org/TR/xquery-operators/#func-codepoints-to-string) | Creates an xs:string from a sequence of Unicode code points.              |
| [string-to-codepoints](https://www.w3.org/TR/xquery-operators/#func-string-to-codepoints) | Returns the sequence of Unicode code points that constitute an xs:string. |

### Equality and Comparison of Strings


| Function                                                                        | Meaning                                                                                                                                                                                                                |
| ------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [compare](https://www.w3.org/TR/xquery-operators/#func-compare)                 | Returns -1, 0, or 1, depending on whether the value of the first argument is respectively less than, equal to, or greater than the value of the second argument, according to the rules of the collation that is used. |
| [codepoint-equal](https://www.w3.org/TR/xquery-operators/#func-codepoint-equal) | Returns true if the two arguments are equal using the Unicode code point collation.                                                                                                                                    |


### Functions on String Values

| Function                                                                        | Meaning                                                                                                                                                                             |
| ------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [concat](https://www.w3.org/TR/xquery-operators/#func-concat)                   | Concatenates two or more xs:anyAtomicType arguments cast to xs:string.                                                                                                              |
| [string-join](https://www.w3.org/TR/xquery-operators/#func-string-join)         | Returns the xs:string produced by concatenating a sequence of xs:strings using an optional separator.                                                                               |
| [substring](https://www.w3.org/TR/xquery-operators/#func-substring)             | Returns the xs:string located at a specified place within an argument xs:string.                                                                                                    |
| [string-length](https://www.w3.org/TR/xquery-operators/#func-string-length)     | Returns the length of the argument.                                                                                                                                                 |
| [normalize-space](https://www.w3.org/TR/xquery-operators/#func-normalize-space) | Returns the whitespace-normalized value of the argument.                                                                                                                            |
| [upper-case](https://www.w3.org/TR/xquery-operators/#func-upper-case)           | Returns the upper-cased value of the argument.                                                                                                                                      |
| [lower-case](https://www.w3.org/TR/xquery-operators/#func-lower-case)           | Returns the lower-cased value of the argument.                                                                                                                                      |
| [translate](https://www.w3.org/TR/xquery-operators/#func-translate)             | Returns the first xs:string argument with occurrences of characters contained in the second argument replaced by the character at the corresponding position in the third argument. |


### Functions Based on Substring Matching

| Function                                                                          | Meaning                                                                                                                                             |
| --------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| [contains](https://www.w3.org/TR/xquery-operators/#func-contains)                 | Indicates whether one xs:string contains another xs:string. A collation may be specified.                                                           |
| [starts-with](https://www.w3.org/TR/xquery-operators/#func-starts-with)           | Indicates whether the value of one xs:string begins with the collation units of another xs:string. A collation may be specified.                    |
| [ends-with](https://www.w3.org/TR/xquery-operators/#func-ends-with)               | Indicates whether the value of one xs:string ends with the collation units of another xs:string. A collation may be specified.                      |
| [substring-before](https://www.w3.org/TR/xquery-operators/#func-substring-before) | Returns the collation units of one xs:string that precede in that xs:string the collation units of another xs:string. A collation may be specified. |
| [substring-after](https://www.w3.org/TR/xquery-operators/#func-substring-after)   | Returns the collation units of xs:string that follow in that xs:string the collation units of another xs:string. A collation may be specified.      |



### String Functions that Use Pattern Matching

| Function                                                          | Meaning                                                                                                                                                                                                             |
| ----------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [matches](https://www.w3.org/TR/xquery-operators/#func-matches)   | Returns an xs:boolean value that indicates whether the value of the first argument is matched by the regular expression that is the value of the second argument.                                                   |
| [replace](https://www.w3.org/TR/xquery-operators/#func-replace)   | Returns the value of the first argument with every substring matched by the regular expression that is the value of the second argument replaced by the replacement string that is the value of the third argument. |
| [tokenize](https://www.w3.org/TR/xquery-operators/#func-tokenize) | Returns a sequence of one or more xs:strings whose values are substrings of the value of the first argument separated by substrings that match the regular expression that is the value of the second argument.     |


###  Additional Boolean Constructor Functions

| Function                                                    | Meaning                                  |
| ----------------------------------------------------------- | ---------------------------------------- |
| [true](https://www.w3.org/TR/xquery-operators/#func-true)   | Constructs the xs:boolean value 'true'.  |
| [false](https://www.w3.org/TR/xquery-operators/#func-false) | Constructs the xs:boolean value 'false'. |

### Functions on Boolean Values

| Function                                                | Meaning                                       |
| ------------------------------------------------------- | --------------------------------------------- |
| [not](https://www.w3.org/TR/xquery-operators/#func-not) | Inverts the xs:boolean value of the argument. |


### Component Extraction Functions on Durations, Dates and Times

| Function                                                                            | Meaning                                    |
| ----------------------------------------------------------------------------------- | ------------------------------------------ |
| [hours-from-time](https://www.w3.org/TR/xquery-operators/#func-hours-from-time)     | Returns the hours from an xs:time value.   |
| [minutes-from-time](https://www.w3.org/TR/xquery-operators/#func-minutes-from-time) | Returns the minutes from an xs:time value. |
| [seconds-from-time](https://www.w3.org/TR/xquery-operators/#func-seconds-from-time) | Returns the seconds from an xs:time value. |


### Functions on Nodes

| Function                                                              | Meaning                                                                                                      |
| --------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| [local-name](https://www.w3.org/TR/xquery-operators/#func-local-name) | Returns the local name of the context node or the specified node as an xs:NCName.                            |
| [number](https://www.w3.org/TR/xquery-operators/#func-number)         | Returns the value of the context item after atomization or the specified argument converted to an xs:double. |

| Function                                                        | Meaning                                                        |
| --------------------------------------------------------------- | -------------------------------------------------------------- |
| [boolean](https://www.w3.org/TR/xquery-operators/#func-boolean) | Computes the effective boolean value of the argument sequence. |
| [empty](https://www.w3.org/TR/xquery-operators/#func-empty)     | Indicates whether or not the provided sequence is empty.       |
| [reverse](https://www.w3.org/TR/xquery-operators/#func-reverse) | Reverses the order of items in a sequence.                     |

| Function                                                    | Meaning                                                         |
| ----------------------------------------------------------- | --------------------------------------------------------------- |
| [count](https://www.w3.org/TR/xquery-operators/#func-count) | Returns the number of items in a sequence.                      |
| [max](https://www.w3.org/TR/xquery-operators/#func-max)     | Returns the maximum value from a sequence of comparable values. |
| [min](https://www.w3.org/TR/xquery-operators/#func-min)     | Returns the minimum value from a sequence of comparable values. |

| Function                                                          | Meaning                                                                                          |
| ----------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| [position](https://www.w3.org/TR/xquery-operators/#func-position) | Returns the position of the context item within the sequence of items currently being processed. |
| [last](https://www.w3.org/TR/xquery-operators/#func-last)         | Returns the number of items in the sequence of items currently being processed.                  |


### Context Functions

| Function                                                                          | Meaning                          |
| --------------------------------------------------------------------------------- | -------------------------------- |
| [current-dateTime](https://www.w3.org/TR/xquery-operators/#func-current-dateTime) | Returns the current xs:dateTime. |
| [current-date](https://www.w3.org/TR/xquery-operators/#func-current-date)         | Returns the current xs:date.     |
| [current-time](https://www.w3.org/TR/xquery-operators/#func-current-time)         | Returns the current xs:time.     |


## Not yet implemented functions

###  Accessors
| Function                                                                  | Accessor     | Accepts                         | Returns                     |
| ------------------------------------------------------------------------- | ------------ | ------------------------------- | --------------------------- |
| [node-name](https://www.w3.org/TR/xquery-operators/#func-node-name)       | node-name    | an optional node                | zero or one xs:QName        |
| [nilled](https://www.w3.org/TR/xquery-operators/#func-nilled)             | nilled       | a node                          | an optional xs:boolean      |
| [data](https://www.w3.org/TR/xquery-operators/#func-data)                 | typed-value  | zero or more items              | a sequence of atomic values |
| [base-uri](https://www.w3.org/TR/xquery-operators/#func-base-uri)         | base-uri     | an optional node or no argument | zero or one xs:anyURI       |
| [document-uri](https://www.w3.org/TR/xquery-operators/#func-document-uri) | document-uri | an optional node                | zero or one xs:anyURI       |


### Functions on Numeric Values

| Function                                                                              | Accessor                                                                                                                                                                                    | Accepts | Returns |
| ------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | ------- |
| [round-half-to-even](https://www.w3.org/TR/xquery-operators/#func-round-half-to-even) | Takes a number and a precision and returns a number rounded to the given precision. If the fractional part is exactly half, the result is the number whose least significant digit is even. |


### Functions on String Values

| Function                                                                            | Meaning                                                                                                                                      |
| ----------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| [normalize-unicode](https://www.w3.org/TR/xquery-operators/#func-normalize-unicode) | Returns the normalized value of the first argument in the normalization form specified by the second argument.                               |
| [encode-for-uri](https://www.w3.org/TR/xquery-operators/#func-encode-for-uri)       | Returns the xs:string argument with certain characters escaped to enable the resulting string to be used as a path segment in a URI.         |
| [iri-to-uri](https://www.w3.org/TR/xquery-operators/#func-iri-to-uri)               | Returns the xs:string argument with certain characters escaped to enable the resulting string to be used as (part of) a URI.                 |
| [escape-html-uri](https://www.w3.org/TR/xquery-operators/#func-escape-html-uri)     | Returns the xs:string argument with certain characters escaped in the manner that html user agents handle attribute values that expect URIs. |




### Functions on anyURI

| Function                                                                | Meaning                                                                                      |
| ----------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| [resolve-uri](https://www.w3.org/TR/xquery-operators/#func-resolve-uri) | Returns an xs:anyURI representing an absolute xs:anyURI given a base URI and a relative URI. |


### Component Extraction Functions on Durations, Dates and Times

| Function                                                                                      | Meaning                                                |
| --------------------------------------------------------------------------------------------- | ------------------------------------------------------ |
| [years-from-duration](https://www.w3.org/TR/xquery-operators/#func-years-from-duration)       | Returns the year component of an xs:duration value.    |
| [months-from-duration](https://www.w3.org/TR/xquery-operators/#func-months-from-duration)     | Returns the months component of an xs:duration value.  |
| [days-from-duration](https://www.w3.org/TR/xquery-operators/#func-days-from-duration)         | Returns the days component of an xs:duration value.    |
| [hours-from-duration](https://www.w3.org/TR/xquery-operators/#func-hours-from-duration)       | Returns the hours component of an xs:duration value.   |
| [minutes-from-duration](https://www.w3.org/TR/xquery-operators/#func-minutes-from-duration)   | Returns the minutes component of an xs:duration value. |
| [seconds-from-duration](https://www.w3.org/TR/xquery-operators/#func-seconds-from-duration)   | Returns the seconds component of an xs:duration value. |
| [year-from-dateTime](https://www.w3.org/TR/xquery-operators/#func-year-from-dateTime)         | Returns the year from an xs:dateTime value.            |
| [month-from-dateTime](https://www.w3.org/TR/xquery-operators/#func-month-from-dateTime)       | Returns the month from an xs:dateTime value.           |
| [day-from-dateTime](https://www.w3.org/TR/xquery-operators/#func-day-from-dateTime)           | Returns the day from an xs:dateTime value.             |
| [hours-from-dateTime](https://www.w3.org/TR/xquery-operators/#func-hours-from-dateTime)       | Returns the hours from an xs:dateTime value.           |
| [minutes-from-dateTime](https://www.w3.org/TR/xquery-operators/#func-minutes-from-dateTime)   | Returns the minutes from an xs:dateTime value.         |
| [seconds-from-dateTime](https://www.w3.org/TR/xquery-operators/#func-seconds-from-dateTime)   | Returns the seconds from an xs:dateTime value.         |
| [timezone-from-dateTime](https://www.w3.org/TR/xquery-operators/#func-timezone-from-dateTime) | Returns the timezone from an xs:dateTime value.        |
| [year-from-date](https://www.w3.org/TR/xquery-operators/#func-year-from-date)                 | Returns the year from an xs:date value.                |
| [month-from-date](https://www.w3.org/TR/xquery-operators/#func-month-from-date)               | Returns the month from an xs:date value.               |
| [day-from-date](https://www.w3.org/TR/xquery-operators/#func-day-from-date)                   | Returns the day from an xs:date value.                 |
| [timezone-from-date](https://www.w3.org/TR/xquery-operators/#func-timezone-from-date)         | Returns the timezone from an xs:date value.            |
| [timezone-from-time](https://www.w3.org/TR/xquery-operators/#func-timezone-from-time)         | Returns the timezone from an xs:time value.            |

###  Timezone Adjustment Functions on Dates and Time Values

| Function                                                                                                | Meaning                                                                        |
| ------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| [adjust-dateTime-to-timezone](https://www.w3.org/TR/xquery-operators/#func-adjust-dateTime-to-timezone) | Adjusts an xs:dateTime value to a specific timezone, or to no timezone at all. |
| [adjust-date-to-timezone](https://www.w3.org/TR/xquery-operators/#func-adjust-date-to-timezone)         | Adjusts an xs:date value to a specific timezone, or to no timezone at all.     |
| [adjust-time-to-timezone](https://www.w3.org/TR/xquery-operators/#func-adjust-time-to-timezone)         | Adjusts an xs:time value to a specific timezone, or to no timezone at all.     |


### Additional Constructor Functions for QNames

| Function                                                                    | Meaning                                                                                                                                          |
| --------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| [resolve-QName](https://www.w3.org/TR/xquery-operators/#func-resolve-QName) | Returns an xs:QName with the lexical form given in the first argument. The prefix is resolved using the in-scope namespaces for a given element. |
| [QName](https://www.w3.org/TR/xquery-operators/#func-qname)                 | Returns an xs:QName with the namespace URI given in the first argument and the local name and prefix in the second argument.                     |


### Functions Related to QNames

| Function                                                                                          | Meaning                                                                                                                      |
| ------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| [prefix-from-QName](https://www.w3.org/TR/xquery-operators/#func-prefix-from-QName)               | Returns an xs:NCName representing the prefix of the xs:QName argument.                                                       |
| [local-name-from-QName](https://www.w3.org/TR/xquery-operators/#func-local-name-from-QName)       | Returns an xs:NCName representing the local name of the xs:QName argument.                                                   |
| [namespace-uri-from-QName](https://www.w3.org/TR/xquery-operators/#func-namespace-uri-from-QName) | Returns the namespace URI for the xs:QName argument. If the xs:QName is in no namespace, the zero-length string is returned. |
| [namespace-uri-for-prefix](https://www.w3.org/TR/xquery-operators/#func-namespace-uri-for-prefix) | Returns the namespace URI of one of the in-scope namespaces for the given element, identified by its namespace prefix.       |
| [in-scope-prefixes](https://www.w3.org/TR/xquery-operators/#func-in-scope-prefixes)               | Returns the prefixes of the in-scope namespaces for the given element.                                                       |


### Functions on Nodes

| Function                                                                    | Meaning                                                                                                                                                                                                                         |
| --------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [name](https://www.w3.org/TR/xquery-operators/#func-name)                   | Returns the name of the context node or the specified node as an xs:string.                                                                                                                                                     |
| [namespace-uri](https://www.w3.org/TR/xquery-operators/#func-namespace-uri) | Returns the namespace URI as an xs:anyURI for the xs:QName of the argument node or the context node if the argument is omitted. This may be the URI corresponding to the zero-length string if the xs:QName is in no namespace. |
| [lang](https://www.w3.org/TR/xquery-operators/#func-lang)                   | Returns true or false, depending on whether the language of the given node or the context node, as defined using the xml:lang attribute, is the same as, or a sublanguage of, the language specified by the argument.           |
| [root](https://www.w3.org/TR/xquery-operators/#func-root)                   | Returns the root of the tree to which the node argument belongs.                                                                                                                                                                |

| Function                                                                        | Meaning                                                                                                                                                                                                                                                                                             |
| ------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [index-of](https://www.w3.org/TR/xquery-operators/#func-index-of)               | Returns a sequence of xs:integers, each of which is the index of a member of the sequence specified as the first argument that is equal to the value of the second argument. If no members of the specified sequence are equal to the value of the second argument, the empty sequence is returned. |
| [exists](https://www.w3.org/TR/xquery-operators/#func-exists)                   | Indicates whether or not the provided sequence is not empty.                                                                                                                                                                                                                                        |
| [distinct-values](https://www.w3.org/TR/xquery-operators/#func-distinct-values) | Returns a sequence in which all but one of a set of duplicate values, based on value equality, have been deleted. The order in which the distinct values are returned is implementation dependent.                                                                                                  |
| [insert-before](https://www.w3.org/TR/xquery-operators/#func-insert-before)     | Inserts an item or sequence of items at a specified position in a sequence.                                                                                                                                                                                                                         |
| [remove](https://www.w3.org/TR/xquery-operators/#func-remove)                   | Removes an item from a specified position in a sequence.                                                                                                                                                                                                                                            |
| [subsequence](https://www.w3.org/TR/xquery-operators/#func-subsequence)         | Returns the subsequence of a given sequence, identified by location.                                                                                                                                                                                                                                |
| [unordered](https://www.w3.org/TR/xquery-operators/#func-unordered)             | Returns the items in the given sequence in a non-deterministic order.                                                                                                                                                                                                                               |

### Functions That Test the Cardinality of Sequences

| Function                                                                | Meaning                                                                                 |
| ----------------------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| [zero-or-one](https://www.w3.org/TR/xquery-operators/#func-zero-or-one) | Returns the input sequence if it contains zero or one items. Raises an error otherwise. |
| [one-or-more](https://www.w3.org/TR/xquery-operators/#func-one-or-more) | Returns the input sequence if it contains one or more items. Raises an error otherwise. |
| [exactly-one](https://www.w3.org/TR/xquery-operators/#func-exactly-one) | Returns the input sequence if it contains exactly one item. Raises an error otherwise.  |


### Equals, Union, Intersection and Except

| Function                                                              | Meaning                                                                                     |
| --------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| [deep-equal](https://www.w3.org/TR/xquery-operators/#func-deep-equal) | Returns true if the two arguments have items that compare equal in corresponding positions. |

###  Aggregate Functions

| Function                                                | Meaning                                      |
| ------------------------------------------------------- | -------------------------------------------- |
| [avg](https://www.w3.org/TR/xquery-operators/#func-avg) | Returns the average of a sequence of values. |
| [sum](https://www.w3.org/TR/xquery-operators/#func-sum) | Returns the sum of a sequence of values.     |


### Functions and Operators that Generate Sequences

| Function                                                                    | Meaning                                                                                                                |
| --------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| [id](https://www.w3.org/TR/xquery-operators/#func-id)                       | Returns the sequence of element nodes having an ID value matching the one or more of the supplied IDREF values.        |
| [idref](https://www.w3.org/TR/xquery-operators/#func-idref)                 | Returns the sequence of element or attribute nodes with an IDREF value matching one or more of the supplied ID values. |
| [doc](https://www.w3.org/TR/xquery-operators/#func-doc)                     | Returns a document node retrieved using the specified URI.                                                             |
| [doc-available](https://www.w3.org/TR/xquery-operators/#func-doc-available) | Returns true if a document node can be retrieved using the specified URI.                                              |
| [collection](https://www.w3.org/TR/xquery-operators/#func-collection)       | Returns a sequence of nodes retrieved using the specified URI or the nodes in the default collection.                  |

### Context Functions

| Function                                                                            | Meaning                                                                       |
| ----------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| [implicit-timezone](https://www.w3.org/TR/xquery-operators/#func-implicit-timezone) | Returns the value of the implicit timezone property from the dynamic context. |
| [default-collation](https://www.w3.org/TR/xquery-operators/#func-default-collation) | Returns the value of the default collation property from the static context.  |
| [static-base-uri](https://www.w3.org/TR/xquery-operators/#func-static-base-uri)     | Returns the value of the Base URI property from the static context.           |

