package main

import (
	"flag"
	"log"
	"os"

	load "github.com/evanofslack/nats-load-traffic"
)

func main() {

	var configPath = flag.String("c", "config.yaml", "path to config file")
	var payloadPath = flag.String("p", "msg.json", "path to message payload file")

	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) != 0 {
		showUsageAndExit(1)
	}
	load.Run(*configPath, *payloadPath)
}

func usage() {
	log.Printf("Usage: nats-load-traffic [-c config path] [-p payload path]\n")
	flag.PrintDefaults()
}

func showUsageAndExit(exitcode int) {
	usage()
	os.Exit(exitcode)
}
