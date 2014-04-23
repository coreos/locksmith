package main

import (
	"fmt"
	"os"

	"github.com/philips/focaccia/lock"
)

var (
	cmdStatus = &Command{
		Name:        "status",
		Summary:     "Get the status of the cluster wide reboot lock.",
		Description: `Status will return the number of locks that are held and available and a list of the holders.`,
		Run:         runStatus,
	}
)

func printHolders(sem *lock.Semaphore) {
	fmt.Fprintln(out, "MACHINE ID")
	for _, h := range sem.Holders {
		fmt.Fprintln(out, h)
	}
}

func runStatus(args []string) (exit int) {
	elc, _ := lock.NewEtcdLockClient(nil)
	l := lock.New("", elc)

	sem, err := l.Get()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error setting value: %s", err)
	}

	fmt.Println("Available:", sem.Semaphore)
	fmt.Println("Max:", sem.Semaphore)

	if len(sem.Holders) > 0 {
		fmt.Fprintln(out, "")
		printHolders(sem)
	}

	return
}
