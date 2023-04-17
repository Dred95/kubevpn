package main

import (
	_ "net/http/pprof"

	"github.com/dred95/kubevpn/cmd/kubevpn/cmds"
)

func main() {
	_ = cmds.NewKubeVPNCommand().Execute()
}
