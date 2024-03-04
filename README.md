# gopom - a maven pom.xml parser

![Tests](https://github.com/chainguard-dev/gopom/workflows/Tests/badge.svg)
![Go Report Card](https://goreportcard.com/badge/github.com/chainguard-dev/gopom)

gopom is a Golang module to easily parse and work with maven pom.xml files.

Supports the offical pom.xml structure that can be read about [here](https://maven.apache.org/ref/3.6.3/maven-model/maven.html).

# Modifications and why

This is forked from: github.com/2000Slash/gopom, but with the mod to round trip the Configuration correctly (for example Plugin.Configuration). Correctly here means it is not worrying about turning it into modifiable struct, but that the entire pom roundtrips correctly (parse, modify some bits, then marshal), should produce a diff that makes sense. With the upstream, it drops a bunch of stuff unless it's a strict key/value list.
This version also handles ordering correctly in a few places.


## Installation

```bash
go get -u github.com/chainguard-dev/gopom
```


## Usage

### Unmarshaling

To load and parse a pom.xml file it is possible to use the `gopom.Parse(path string)` function which will load the file at the given path and return the parsed pom.
See below for example:
```go
package main

import (
	"github.com/chainguard-dev/gopom"
	"log"
)

func main() {

	var pomPath string = ... // Path to the pom.xml file
	parsedPom, err := gopom.Parse(pomPath)
	if err != nil {
		log.Fatal(err)
	}
}
```

If one already has the pom.xml loaded as a string or bytes you can use `encoding/xml` from the standard library.
This can be seen below:
```go
package main

import (
	"encoding/xml"
	"github.com/chainguard-dev/gopom"
	"log"
)

func main() {
	var pomString string = ... // The pom string

	var parsedPom gopom.Project
	err := xml.Unmarshal([]byte(pomString), &parsedPom)
	if err != nil {
		log.Fatal(err)
	}
}
```

### Marshaling

You can also marshal the project back to an xml, and for that you *MUST*
use the `Marshal` function, because of the way things are named, and
handled.

```go
package main

import (
	"fmt"
	"log"

	"github.com/chainguard-dev/gopom"
)

func main() {

	var pomPath string = "./pom.xml"
	parsedPom, err := gopom.Parse(pomPath)
	if err != nil {
		log.Fatal(err)
	}
	output, _ := parsedPom.Marshal()
	fmt.Println(string(output))
}
```


## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License

Original work Copyright (c) 2020-present [Viktor Franzén](https://github.com/vifraa)
Modified work Copyright (c) 2021 [Nils Hartmann](https://github.com/2000Slash)

Licensed under [MIT License](./LICENSE)
