package endpointc

import (
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	// local listen tcp port
	localPort uint16 //  = 8009
	// remote port, that endpoint server
	// should connect to
	remotePort uint16 // = 3389
	// device uuid
	deviceID string
	// quic server addr
	quicAddr string

	// map keep all current websocket
	// use for keep-alive
	holderMap = make(map[string]*sessionholder)
)

// Params parameters
type Params struct {
	// local listen tcp port
	LocalPort uint16
	// remote port, that endpoint server
	// should connect to
	RemotePort uint16
	// device uuid
	UUID string
	// quic server addr
	QuicAddr string
}

// keepalive send ping to all websocket holder
func keepalive() {
	for {
		time.Sleep(time.Second * 20)

		for _, v := range holderMap {
			v.keepalive()
		}
	}
}

// Run run endpoint client and
// wait client to connect
func Run(params *Params) {
	localPort = params.LocalPort
	remotePort = params.RemotePort
	deviceID = params.UUID
	quicAddr = params.QuicAddr

	log.Printf("endpoint run, local port:%d, target port:%d, device uuid:%s", localPort, remotePort, deviceID)

	// keep-alive goroutine
	go keepalive()

	startTCPListener(localPort)
}
