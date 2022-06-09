package main

import (
	_ "embed"
	"github.com/Renloi/Renloi/command/root"
	"github.com/Renloi/Renloi/licenses"
)

var (
	//go:embed LICENSE
	license string
)

func main() {
	licenses.SetLicense(license)

	root.NewRootCommand().Execute()
}
