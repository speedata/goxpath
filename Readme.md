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

## Implementation status

The implementation is about 50% complete. The following XPath functions are implemented:

* abs
* boolean
* ceiling
* codepoints-to-string
* concat
* contains
* count
* empty
* false
* floor
* last
* local-name
* lower-case
* matches
* max
* min
* not
* normalize-space
* number
* position
* replace
* reverse
* round
* string
* string-join
* string-length
* string-to-codepoints
* substring
* true
* tokenize
* upper-case
