// SPDX-License-Identifier: GPL-3.0-only
package main

import (
	"fmt"
	"lumin/pkg/pluginapi"
	"os"
)

func main() {
	t := newTranslator()
	if err := pluginapi.Serve(t.translate); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
