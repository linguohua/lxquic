package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"

	"lxquic/endpoints"
	"lxquic/wait"
)

var (
	uuid     string
	quicAddr string
	daemon   = ""
)

func init() {
	flag.StringVar(&uuid, "u", "", "specify device uuid")
	flag.StringVar(&quicAddr, "addr", "", "specify quic server address")
	flag.StringVar(&daemon, "d", "yes", "specify daemon mode")
}

// getVersion get version
func getVersion() string {
	return "0.1.0"
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

	log.Println("try to start  lxquic endpoint server, version:", getVersion())

	if uuid == "" {
		log.Fatal("please specify device uuid")
	}

	if quicAddr == "" {
		log.Fatal("please specify quic server address")
	}

	params := &endpoints.Params{
		UUID:     uuid,
		QuicAddr: quicAddr,
	}

	// start http server
	go endpoints.Run(params)
	log.Println("start lxquic endpoint server ok!")

	if daemon == "yes" {
		wait.GetSignal()
	} else {
		wait.GetInput()
	}
	return
}
