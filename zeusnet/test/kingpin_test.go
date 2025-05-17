package test

import (
	"gopkg.in/alecthomas/kingpin.v2"
	"testing"
)

var (
	app = kingpin.New("test", "A test app")

	version = app.Command("version", "Print version information")
)

func TestKingPin(t *testing.T) {
	// 实际上要换成命令行参数 os.Args[1:]
	fullCmd := kingpin.MustParse(app.Parse([]string{"version"}))
	if fullCmd == version.FullCommand() {
		t.Log("version command")
	}
}
