package main

import (
	"github.com/hoangtrungnguyen/grava/pkg/cmd"
)

var Version string

func main() {
	cmd.SetVersion(Version)
	cmd.Execute()
}
