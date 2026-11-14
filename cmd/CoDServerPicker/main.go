// Command CoDServerPicker is the desktop app of CoD Server Picker.
package main

import "github.com/1ukah/cod-server-picker/cmd/CoDServerPicker/ui"

// version is set at build time with -ldflags "-X main.version=X.Y.Z".
var version = "dev"

func main() {
	ui.Run(version)
}
