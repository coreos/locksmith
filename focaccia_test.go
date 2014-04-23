package main

import (
	"testing"
)

func TestWatch(t *testing.T) {
	var ch chan bool
	ch <- true
	go watch(ch)
}
