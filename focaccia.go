package main

import (
	"github.com/coreos/go-etcd/etcd"
	"github.com/philips/focaccia/updateengine"
)

const (
	rebootKey = "coreos.com/updateengine/rebootlock"
)

func main() {
	var ch chan bool

	ec := etcd.NewClient(nil)
	ue, err := updateengine.New()
	if err != nil {
		panic(err)
	}

	go ue.RebootNeededSignal(ch)

	needed := <-ch
	if needed {
		println("Reboot needed")
		ec.Create(rebootKey, "bootid", 0)
	}
}
