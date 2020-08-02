package server

import (
	"context"
	"encoding/json"
	"fmt"
	"lxquic/protoj"
	"net"

	"github.com/lucas-clemente/quic-go"
	log "github.com/sirupsen/logrus"
)

type pxEndpoint struct {
	index int

	proxyToken string

	sess   quic.Session
	stream quic.Stream

	waitingPingCount int
}

// keepalive send ping message peer, and counter
func (ee *pxEndpoint) keepalive() {
	if ee.stream == nil {
		log.Println("pxEndpoint.keepalive ee.stream == nil")
		return
	}

	// too many un-response ping, close the websocket connection
	if ee.waitingPingCount > 3 {
		if ee.sess != nil {
			log.Println("pxEndpoint.keepalive keepalive failed, close, index:", ee.index)
			ee.sess.CloseWithError(0, "keepalive failed")
		}
		return
	}

	var ping = &protoj.StreamCmd{
		Cmd: "ping",
	}

	err := protoj.StreamSendJSON(ee.stream, ping)
	if err != nil {
		log.Println("pxEndpoint.keepalive streamSendJSON ping error:", err)
		return
	}

	ee.waitingPingCount++
}

// onPong update waiting response ping counter
func (ee *pxEndpoint) onPong(data []byte) {
	ee.waitingPingCount = 0
}

func (ee *pxEndpoint) serveCmdStream() {
	stream := ee.stream
	for {
		message, err := protoj.StreamReadJSON(stream)
		if err != nil {
			log.Println("pxEndpoint.serveCmdStream streamReadJSON failed:", err)
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
			log.Println("pxEndpoint.serveCmdStream json.Unmarshal failed:", err)
		}
	}
}

func servePX(sess quic.Session, stream quic.Stream, header *protoj.CmdStreamHeader) {
	log.Printf("servePX, got a px endpoint:%+v", header)
	if header.DUID != proxyToken {
		log.Printf("servePX proxy token not match %s != %s", header.DUID, proxyToken)
		return
	}

	px := &pxEndpoint{
		index:      ecIndex,
		proxyToken: header.DUID,
		sess:       sess,
		stream:     stream,
	}

	ecIndex++

	pxmap[px.index] = px

	defer func() {
		delete(pxmap, px.index)
	}()

	go px.serveCmdStream()
	px.acceptPXStream()
}

func (ee *pxEndpoint) acceptPXStream() {
	log.Println("pxEndpoint.acceptLinkStream wait link stream")
	sess := ee.sess
	for {
		ecStream, err := sess.AcceptStream(context.Background())
		if err != nil {
			log.Println("pxEndpoint.acceptLinkStream sess.AcceptStream failed:", err)
			return
		}

		go servePXStream(ecStream)
	}
}

func servePXStream(stream quic.Stream) {
	defer stream.Close()

	// read LinkStreamHeader
	message, err := protoj.StreamReadJSON(stream)
	if err != nil {
		log.Errorf("servePXStream streamReadJSON failed:%v", err)
		return
	}

	var header = &protoj.LinkStreamHeader{}
	err = json.Unmarshal(message, header)
	if err != nil {
		log.Errorf("servePXStream json.UnMarshal failed:%v", err)
		return
	}

	// only allow connect to local host
	address := fmt.Sprintf("%s:%d", header.Host, header.Port)

	log.Printf("servePXStream, try link to:%s", address)

	// connect to local network via tcp
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Errorf("servePXStream connect to address:%s failed:%v", address, err)
		return
	}

	// ensure the tcp connection will closed final
	defer conn.Close()
	quicbuf := make([]byte, 64*1024)

	// receive stream message and forward to tcp
	go func() {
		defer conn.Close()
		for {
			n, err := stream.Read(quicbuf)
			if err != nil {
				log.Println("servePXStream quic stream read error:", err)
				break
			}

			if n == 0 {
				break
			}

			protoj.WriteAll(conn, quicbuf[0:n])
			if err != nil {
				break
			}
		}
	}()

	// receive tcp message and forward to websocket
	tcpbuf := make([]byte, 64*1024)

	for {
		n, err := conn.Read(tcpbuf)
		if err != nil {
			log.Println("servePXStream tcp read error:", err)
			break
		}

		//log.Printf("servePXStream, read tcp bytes:%d", n)
		_, err = stream.Write(tcpbuf[:n])
		if err != nil {
			log.Println("servePXStream ws write error:", err)
			break
		}
	}

	log.Println("servePXStream, pair link stream end")
}
