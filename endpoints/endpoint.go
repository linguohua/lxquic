package endpoints

import (
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	// the device id
	deviceID string
	// quic server address
	quicAddr string
	// base websocket url

	// map keep all current websocket
	// use for keep-alive
	holderMap = make(map[string]*sessionholder)
)

// Params parameters
type Params struct {
	// device id
	UUID string
	// quic server address
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

// Run run endpoint server and
// wait for server's command
func Run(params *Params) {
	deviceID = params.UUID
	quicAddr = params.QuicAddr

	// keep-alive goroutine
	go keepalive()

	log.Printf("endpoint run, device uuid:%s", deviceID)
	cmdwsService()
}
