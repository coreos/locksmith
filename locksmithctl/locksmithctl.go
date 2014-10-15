package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"text/tabwriter"

	"github.com/coreos/locksmith/etcd"
	"github.com/coreos/locksmith/lock"
)

const (
	cliName        = "locksmithctl"
	cliDescription = `Manage the cluster wide reboot lock.`
)

var (
	out *tabwriter.Writer

	commands      []*Command
	globalFlagset *flag.FlagSet = flag.NewFlagSet("locksmithctl", flag.ExitOnError)

	globalFlags = struct {
		Debug        bool
		Endpoint     string
		EtcdKeyFile  string
		EtcdCertFile string
		EtcdCAFile   string
	}{}
)

func init() {
	out = new(tabwriter.Writer)
	out.Init(os.Stdout, 0, 8, 1, '\t', 0)

	globalFlagset.BoolVar(&globalFlags.Debug, "debug", false, "Print out debug information to stderr.")
	globalFlagset.StringVar(&globalFlags.Endpoint, "endpoint", "http://127.0.0.1:4001", "etcd endpoint for locksmith. Defaults to the local instance.")
	globalFlagset.StringVar(&globalFlags.EtcdKeyFile, "etcd-keyfile", "", "etcd key file authentication")
	globalFlagset.StringVar(&globalFlags.EtcdCertFile, "etcd-certfile", "", "etcd cert file authentication")
	globalFlagset.StringVar(&globalFlags.EtcdCAFile, "etcd-cafile", "", "etcd CA file authentication")

	commands = []*Command{
		cmdHelp,
		cmdLock,
		cmdReboot,
		cmdSendNeedReboot,
		cmdSetMax,
		cmdStatus,
		cmdUnlock,
	}
}

type Command struct {
	Name        string                  // Name of the Command and the string to use to invoke it
	Summary     string                  // One-sentence summary of what the Command does
	Usage       string                  // Usage options/arguments
	Description string                  // Detailed description of command
	Flags       flag.FlagSet            // Set of flags associated with this command
	Run         func(args []string) int // Run a command with the given arguments, return exit status
}

func getAllFlags() (flags []*flag.Flag) {
	return getFlags(globalFlagset)
}

func getFlags(flagset *flag.FlagSet) (flags []*flag.Flag) {
	flags = make([]*flag.Flag, 0)
	flagset.VisitAll(func(f *flag.Flag) {
		flags = append(flags, f)
	})
	return
}

func main() {
	globalFlagset.Parse(os.Args[1:])
	var args = globalFlagset.Args()

	// no command specified - trigger help
	if len(args) < 1 {
		args = append(args, "help")
	}

	progName := os.Args[0]
	if path.Base(progName) == "locksmithd" {
		os.Exit(runDaemon([]string{}))
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

// getLockClient returns an initialized EtcdLockClient, using an etcd
// client configured from the global etcd flags
func getClient() (*lock.EtcdLockClient, error) {
	var ti *etcd.TLSInfo
	if globalFlags.EtcdCAFile != "" || globalFlags.EtcdCertFile != "" || globalFlags.EtcdKeyFile != "" {
		ti = &etcd.TLSInfo{
			CertFile: globalFlags.EtcdCertFile,
			KeyFile:  globalFlags.EtcdKeyFile,
			CAFile:   globalFlags.EtcdCAFile,
		}
	}
	ec, err := etcd.NewClient([]string{globalFlags.Endpoint}, ti)
	if err != nil {
		return nil, err
	}
	lc, err := lock.NewEtcdLockClient(ec)
	if err != nil {
		return nil, err
	}
	return lc, err
}
