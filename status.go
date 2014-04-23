package main

import (
	"fmt"
	"os"

	"github.com/philips/focaccia/lock"
)

var (
	cmdStatus = &Command{
		Name:    "status",
		Summary: "Get the status of the cluster wide reboot lock.",
		Usage:   "UNIT",
		Description:
`Status will return the number of locks that are held and available and a list of the holders.`,
		Run: runStatus,
	}
)

func runStatus(args []string) (exit int) {
	elc, _ := lock.NewEtcdLockClient(nil)
	l := lock.New("hi", elc)

	sem, err := l.Get()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error setting value: %s", err)
	}

	fmt.Println("Available:", sem.Semaphore)
	fmt.Println("Max:", sem.Semaphore)

	

	return
}
