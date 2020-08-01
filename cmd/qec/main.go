package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"

	"lxquic/endpointc"
	"lxquic/wait"
)

var (
	lport    int
	rport    int
	uuid     string
	quicAddr string
	daemon   = ""
)

func init() {
	flag.IntVar(&lport, "l", 8009, "specify the listen port")
	flag.IntVar(&rport, "r", 3389, "specify target port")
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

	log.Println("try to start  lxquic endpoint client, version:", getVersion())

	if uuid == "" {
		log.Fatal("please specify target device uuid")
	}

	if quicAddr == "" {
		log.Fatal("please specify quic server addr")
	}

	params := &endpointc.Params{
		LocalPort:  uint16(lport),
		RemotePort: uint16(rport),
		UUID:       uuid,
		QuicAddr:   quicAddr,
	}

	// start http server
	go endpointc.Run(params)
	log.Println("start lxquic endpoint client ok!")

	if daemon == "yes" {
		wait.GetSignal()
	} else {
		wait.GetInput()
	}
	return
}
