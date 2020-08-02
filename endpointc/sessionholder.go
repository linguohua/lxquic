package endpointc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"lxquic/protoj"

	"github.com/lucas-clemente/quic-go"

	log "github.com/sirupsen/logrus"
)

// sessionholder websocket holder
type sessionholder struct {
	uuid string
	sess quic.Session

	stream quic.Stream

	// ping meesage that waiting for response counter
	waitingPingCount int
}

// newHolder create a websocket holder object
// the uuid must be unique
func newHolder(uuid string, sess1 quic.Session, stream1 quic.Stream) *sessionholder {
	wh := &sessionholder{
		uuid:   uuid,
		sess:   sess1,
		stream: stream1,
	}

	return wh
}

func getHolder(uid string) *sessionholder {
	h, _ := holderMap[uid]

	return h
}

func buildQuicConnection(role string, uid string) *sessionholder {
	log.Println("buildQuicConnection")

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}

	// build websocket connection
	session, err := quic.DialAddr(quicAddr, tlsConf, nil)
	if err != nil {
		log.Println("handleRequest quic.DialAddr failed:", err)
		return nil
	}

	cmdStream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		log.Println("handleRequest session.OpenStreamSync failed:", err)
		return nil
	}

	var header = &protoj.CmdStreamHeader{
		Role: role,
		DUID: uid,
		Port: int(remotePort),
	}

	err = protoj.StreamSendJSON(cmdStream, header)
	if err != nil {
		log.Println("handleRequest streamSendJSON failed:", err)
		return nil
	}

	var holder = newHolder(uid, session, cmdStream)
	holderMap[uid] = holder

	go holder.serveCmdStream()

	return holder
}

func (wh *sessionholder) keepalive() {
	if wh.stream == nil {
		return
	}

	// too many un-response ping, close the websocket connection
	if wh.waitingPingCount > 3 {
		if wh.sess != nil {
			log.Println("sessionholder keepalive failed, close")
			wh.sess.CloseWithError(0, "keepalive failed")
		}
		return
	}

	var ping = &protoj.StreamCmd{
		Cmd: "ping",
	}

	err := protoj.StreamSendJSON(wh.stream, ping)
	if err != nil {
		log.Println("streamSendJSON ping error:", err)
		return
	}

	wh.waitingPingCount++
}

// onPong update waiting response ping counter
func (wh *sessionholder) onPong(data []byte) {
	wh.waitingPingCount = 0
}

func (wh *sessionholder) serveCmdStream() {
	stream := wh.stream
	// remove from map
	defer delete(holderMap, wh.uuid)

	for {
		message, err := protoj.StreamReadJSON(stream)
		if err != nil {
			log.Println("serveCmdStream streamReadJSON failed:", err)
			break
		}

		var cmd = &protoj.StreamCmd{}
		err = json.Unmarshal(message, cmd)
		if err == nil {
			//log.Printf("sessionholder serveCmdStream get a json cmd:%+v", cmd)
			if cmd.Cmd == "pong" {
				wh.onPong(nil)
			} else if cmd.Cmd == "ping" {
				// reply pong
				cmd.Cmd = "pong"
				protoj.StreamSendJSON(stream, cmd)
			}
		} else {
			//
			log.Println("sessionholder serveCmdStream json.Unmarshal failed:", err)
		}
	}
}
