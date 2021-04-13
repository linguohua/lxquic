package server

import (
	"context"
	"encoding/json"
	"io"
	"lxquic/protoj"

	"github.com/lucas-clemente/quic-go"
	log "github.com/sirupsen/logrus"
)

type ecEndpoint struct {
	index int

	targetPort  int
	targetDevID string

	sess   quic.Session
	stream quic.Stream

	waitingPingCount int
}

// keepalive send ping message peer, and counter
func (ee *ecEndpoint) keepalive() {
	if ee.stream == nil {
		log.Println("ecEndpoint.keepalive ee.stream == nil")
		return
	}

	// too many un-response ping, close the websocket connection
	if ee.waitingPingCount > 3 {
		if ee.sess != nil {
			log.Println("ecEndpoint.keepalive keepalive failed, close, index:", ee.index)
			ee.sess.CloseWithError(0, "keepalive failed")
		}
		return
	}

	var ping = &protoj.StreamCmd{
		Cmd: "ping",
	}

	err := protoj.StreamSendJSON(ee.stream, ping)
	if err != nil {
		log.Println("ecEndpoint.keepalive streamSendJSON ping error:", err)
		return
	}

	ee.waitingPingCount++
}

func serveEC(sess quic.Session, stream quic.Stream, header *protoj.CmdStreamHeader) {
	log.Printf("serveEC, got a ec endpoint:%+v", header)
	ec := &ecEndpoint{
		index:       ecIndex,
		targetDevID: header.DUID,
		targetPort:  header.Port,
		sess:        sess,
		stream:      stream,
	}
	ecIndex++

	ecmap[ec.index] = ec

	defer func() {
		delete(ecmap, ec.index)
	}()

	go ec.serveCmdStream()
	ec.acceptLinkStream()
}

// onPong update waiting response ping counter
func (ee *ecEndpoint) onPong(data []byte) {
	ee.waitingPingCount = 0
}

func (ee *ecEndpoint) serveCmdStream() {
	stream := ee.stream
	for {
		message, err := protoj.StreamReadJSON(stream)
		if err != nil {
			log.Println("ecEndpoint.serveCmdStream streamReadJSON failed:", err)
			break
		}

		var cmd = &protoj.StreamCmd{}
		err = json.Unmarshal(message, cmd)
		if err == nil {
			//log.Printf("ecEndPoint.serveCmdStream get json:%+v", cmd)
			if cmd.Cmd == "pong" {
				ee.onPong(nil)
			} else if cmd.Cmd == "ping" {
				// reply pong
				cmd.Cmd = "pong"
				protoj.StreamSendJSON(stream, cmd)
			}
		} else {
			log.Println("ecEndPoint.serveCmdStream json.Unmarshal failed:", err)
		}
	}
}

func (ee *ecEndpoint) acceptLinkStream() {
	log.Println("ecEndpoint.acceptLinkStream wait link stream")
	sess := ee.sess
	for {
		ecStream, err := sess.AcceptStream(context.Background())
		if err != nil {
			log.Println("ecEndpoint.acceptLinkStream sess.AcceptStream failed:", err)
			return
		}

		es, ok := esmap[ee.targetDevID]
		if !ok {
			log.Printf("ecEndpoint.acceptLinkStream, not device found for:%s, close stream", ee.targetDevID)
			ecStream.Close()
			continue
		}

		go pairEE(ee, es, ecStream)
	}
}

func pairEE(ec *ecEndpoint, es *esEndpoint, ecStream quic.Stream) {
	log.Printf("pairEE ec start link stream, target dev:%s, target port:%d", ec.targetDevID, ec.targetPort)
	defer ecStream.Close()
	sess := es.sess
	if sess == nil {
		log.Println("pairEE, sess is nil, discard")
		return
	}

	esStream, err := sess.OpenStreamSync(context.Background())
	if err != nil {
		log.Printf("pairEE, sess OpenStreamSync failed:%v, discard", err)
		return
	}

	defer esStream.Close()
	log.Printf("sess.OpenStreamSync ok, target dev:%s", ec.targetDevID)

	var header = &protoj.LinkStreamHeader{
		Port: ec.targetPort,
	}

	protoj.StreamSendJSON(esStream, header)
	if err != nil {
		log.Printf("pairEE, esStream StreamSendJSON failed:%v, discard", err)
		return
	}

	log.Printf("protoj.StreamSendJSON ok, target dev:%s", ec.targetDevID)

	go func() {
		defer esStream.Close()
		defer ecStream.Close()

		io.Copy(esStream, ecStream)
	}()

	io.Copy(ecStream, esStream)
	log.Printf("pairEE ec link stream end, target dev:%s, target port:%d", ec.targetDevID, ec.targetPort)
}
