// Command makedmg writes an ISO 9660 / Joliet / Rock Ridge image.
//
// macOS DiskImageMounter opens this format when the file uses a .dmg
// extension, so a distributable disk image can be built on Linux as well
// as with hdiutil on macOS.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	src := flag.String("src", "", "directory to pack (the DMG root)")
	out := flag.String("out", "", "output .dmg path")
	vol := flag.String("volname", "Dogubako", "volume name")
	flag.Parse()
	if *src == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: makedmg -src DIR -out FILE.dmg [-volname NAME]")
		os.Exit(2)
	}
	if err := writeImage(*out, *src, *vol, time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
