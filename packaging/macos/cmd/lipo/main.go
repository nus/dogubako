// Command lipo creates a Mach-O universal binary via github.com/konoui/lipo.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/konoui/lipo/pkg/lipo"
)

func main() {
	out := flag.String("o", "", "output universal binary")
	flag.Parse()
	if err := create(*out, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func create(out string, inputs []string) error {
	if out == "" {
		return fmt.Errorf("usage: lipo -o OUT IN...")
	}
	if len(inputs) < 2 {
		return fmt.Errorf("usage: lipo -o OUT IN...")
	}
	return lipo.New(lipo.WithInputs(inputs...), lipo.WithOutput(out)).Create()
}
