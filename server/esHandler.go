package server

import (
	"encoding/json"
	"lxquic/protoj"
	"sync"

	"github.com/lucas-clemente/quic-go"
	log "github.com/sirupsen/logrus"
)

type esEndpoint struct {
	devID string

	sess   quic.Session
	stream quic.Stream

	waitingPingCount int

	wg sync.WaitGroup
}

func (ee *esEndpoint) close() {
	stream := ee.stream
	if stream != nil {
		stream.Close()
	}

	ss := ee.sess
	if ss != nil {
		ss.CloseWithError(0, "force closed")
	}
}

// keepalive send ping message peer, and counter
func (ee *esEndpoint) keepalive() {
	if ee.stream == nil {
		log.Println("esEndpoint.keepalive ee.stream == nil")
		return
	}

	// too many un-response ping, close the websocket connection
	if ee.waitingPingCount > 3 {
		if ee.sess != nil {
			log.Println("esEndpoint.keepalive keepalive failed, close:", ee.devID)
			ee.sess.CloseWithError(0, "keepalive failed")
		}
		return
	}

	var ping = &protoj.StreamCmd{
		Cmd: "ping",
	}

	err := protoj.StreamSendJSON(ee.stream, ping)
	if err != nil {
		log.Println("esEndpoint.keepalive streamSendJSON ping error:", err)
		return
	}

	ee.waitingPingCount++
}

func serveES(sess quic.Session, stream quic.Stream, header *protoj.CmdStreamHeader) {
	log.Printf("serveES, got a es endpoint:%+v", header)
	es := &esEndpoint{
		devID:  header.DUID,
		sess:   sess,
		stream: stream,
	}

	old, ok := esmap[es.devID]
	if ok {
		log.Println("serveES wait old es endpoint exit:", es.devID)
		old.close()
		// wait
		old.wg.Wait()
		log.Println("serveES wait old es endpoint exit ok:", es.devID)
	}

	es.wg.Add(1)
	esmap[es.devID] = es

	defer func() {
		delete(esmap, es.devID)
		es.wg.Done()
	}()

	es.serveCmdStream()
}

// onPong update waiting response ping counter
func (ee *esEndpoint) onPong(data []byte) {
	ee.waitingPingCount = 0
}

func (ee *esEndpoint) serveCmdStream() {
	stream := ee.stream
	for {
		message, err := protoj.StreamReadJSON(stream)
		if err != nil {
			log.Println("esEndpoint.serveCmdStream streamReadJSON failed:", err)
			break
		}

		var cmd = &protoj.StreamCmd{}
		err = json.Unmarshal(message, cmd)
		if err == nil {
			//log.Printf("esEndPoint.serveCmdStream get json:%+v", cmd)
			if cmd.Cmd == "pong" {
				ee.onPong(nil)
			} else if cmd.Cmd == "ping" {
				// reply pong
				cmd.Cmd = "pong"
				protoj.StreamSendJSON(stream, cmd)
			}
		} else {
			//
			log.Println("esEndPoint.serveCmdStream json.Unmarshal failed:", err)
		}
	}
}
