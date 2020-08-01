package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"

	"lxquic/server"
	"lxquic/wait"
)

var (
	listenAddr = ""
	daemon     = ""
)

func init() {
	flag.StringVar(&listenAddr, "l", ":443", "specify the listen address")
	flag.StringVar(&daemon, "d", "yes", "specify daemon mode")
}

// getVersion get version
func getVersion() string {
	return "0.1.1"
}

func main() {
	// only one thread
	runtime.GOMAXPROCS(1)

	version := flag.Bool("v", false, "show version")

	flag.Parse()

	if *version {
		fmt.Printf("%s\n", getVersion())
		os.Exit(0)
	}

	log.Println("try to start  lxquic server, version:", getVersion())

	params := &server.Params{
		ListenAddr: listenAddr,
	}

	// start http server
	go server.CreateQuicServer(params)
	log.Println("start lxquic server ok!")

	if daemon == "yes" {
		wait.GetSignal()
	} else {
		wait.GetInput()
	}
	return
}
