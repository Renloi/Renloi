package main

import (
	_ "embed"
	"github.com/renloi/Renloi/command/root"
	"github.com/renloi/Renloi/licenses"
)

var (
	//go:embed LICENSE
	license string
)

func main() {
	licenses.SetLicense(license)

	root.NewRootCommand().Execute()
}
