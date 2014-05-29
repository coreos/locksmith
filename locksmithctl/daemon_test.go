package main

import (
	"math"
	"testing"
)

func TestExpBackoff(t *testing.T) {
	interval := initialInterval
	for i := 0; i < math.MaxUint16; i++ {
		interval = expBackoff(interval)
		if interval < 0 {
			t.Fatalf("interval too small: %v %v", interval, i)
		}
		if interval > maxInterval {
			t.Fatalf("interval too large: %v %v", interval, i)
		}
	}
}
