package main

import (
	"flag"
	"fmt"
	"os"
)

const (
	cliName = "rebootlockctl"

)

var (
	commands []*Command
	globalFlagset *flag.FlagSet = flag.NewFlagSet("rebootlockctl", flag.ExitOnError)

	globalFlags = struct {
		Debug bool
	}{}
)

func init() {
	globalFlagset.BoolVar(&globalFlags.Debug, "debug", false, "Print out debug information to stderr.")

	commands = []*Command {
		cmdWatch,
		cmdSetMax,
	}
}

type Command struct {
	Name        string       // Name of the Command and the string to use to invoke it
	Summary     string       // One-sentence summary of what the Command does
	Usage       string       // Usage options/arguments
	Description string       // Detailed description of command
	Flags       flag.FlagSet // Set of flags associated with this command
	Run func(args []string) int // Run a command with the given arguments, return exit status
}


func main() {
	globalFlagset.Parse(os.Args[1:])
	var args = globalFlagset.Args()

	// no command specified - trigger help
	if len(args) < 1 {
		args = append(args, "help")
	}

	var cmd *Command

	// determine which Command should be run
	for _, c := range commands {
		if c.Name == args[0] {
			cmd = c
			if err := c.Flags.Parse(args[1:]); err != nil {
				fmt.Println(err.Error())
				os.Exit(2)
			}
			break
		}
	}

	if cmd == nil {
		fmt.Printf("%v: unknown subcommand: %q\n", cliName, args[0])
		fmt.Printf("Run '%v help' for usage.\n", cliName)
		os.Exit(2)
	}

	os.Exit(cmd.Run(cmd.Flags.Args()))
}
