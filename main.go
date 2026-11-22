// Lumin is a native desktop translation application.
package main

import (
	_ "embed"
	"log"

	"lumin/internal/bootstrap"
)

//go:embed resources/icon.png
var icon []byte

func main() {
	if err := bootstrap.Run(icon); err != nil {
		log.Fatal(err)
	}
}
