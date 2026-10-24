// Command dmgwindow draws the Finder background for the macOS UDZO image
// and writes the icon positions that match that picture.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/nus/dogubako/packaging/macos/dmgwindow"
)

func main() {
	out := flag.String("o", "", "directory for background.png, background@2x.png, and layout.json")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "usage: dmgwindow -o DIR")
		os.Exit(2)
	}
	if err := dmgwindow.WriteDir(*out, dmgwindow.Default()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
