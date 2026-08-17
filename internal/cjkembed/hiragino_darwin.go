//go:build darwin

package cjkembed

import (
	"fmt"
	"os"
)

func init() {
	src, _, err := openHiragino(defaultHiraginoPaths())
	if err != nil {
		fmt.Fprintf(os.Stderr, "dogubako: hiragino: %v\n", err)
		return
	}
	registerHiragino(src)
}
