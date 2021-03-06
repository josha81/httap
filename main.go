package main

import (
	"fmt"
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"

	httap "./lib"
	"github.com/jessevdk/go-flags"
)

type options struct {
	httap.Options
	Help    bool `long:"help"    description:"Display this help and exit."`
	Version bool `long:"version" description:"Display version number and exit."`
}

var parser *flags.Parser
var buildTag string

func writeVersion() {
	cliName := path.Base(os.Args[0])
	cliVersion := "0.2"
	fmt.Fprintf(os.Stderr, "%s version %s-%s (%s, %s)\n", cliName, cliVersion, buildTag,
		httap.PcapVersion(), runtime.Version())
}

func writeHelp() {
	fmt.Fprintln(os.Stderr, "Wiretaps and forwards HTTP traffic\n")
	parser.WriteHelp(os.Stderr)
}

func reportError() {
	if err := recover(); err != nil {
		fmt.Fprintln(os.Stderr, "Fatal:", err)
		os.Exit(1)
	}
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	/* We don't have to read from the signal channel, the signal package
	   promises not to block while sending and the signal will be discarded. */
	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGHUP)
}

func main() {
	defer reportError()

	var opts struct {
		options `group:"Options"`
	}

	parser = flags.NewParser(&opts, flags.None)
	parser.Usage = "[OPTIONS] [--src HOST:PORT ...] --dst HOST:PORT ..."
	_, err := parser.Parse()

	if len(os.Args) == 1 {
		opts.Help = true
	}

	if opts.Version {
		writeVersion()
	} else if opts.Help {
		writeHelp()
	} else if err != nil {
		panic(err)
	} else {
		httap.NewWiretap(opts.Options).Start()
	}
}
