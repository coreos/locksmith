package main

import (
	"github.com/philips/focaccia/updateengine"
)

func main() {
	var ch chan bool

	ue, err := updateengine.New()
	if err != nil {
		panic(err)
	}

	println(ue.GetStatus())
	go ue.RebootNeededSignal(ch)

	needed := <-ch
	if needed {
		println("Reboot needed")
	}
}
