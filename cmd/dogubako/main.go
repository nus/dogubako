package main

import (
	"fmt"
	"os"

	"github.com/nus/dogubako/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
