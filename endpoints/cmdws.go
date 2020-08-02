package endpoints

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"

	quic "github.com/lucas-clemente/quic-go"

	"lxquic/protoj"
)

type sessionholder struct {
	// unique id, as sessionholder object's identifier
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

// buildCmdWS build a websocket dedicated to recv command
func buildCmdWS() (*sessionholder, error) {
	log.Println("buildCmdWS")
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}
	session, err := quic.DialAddr(quicAddr, tlsConf, nil)
	if err != nil {
		return nil, err
	}

	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		return nil, err
	}

	var header = &protoj.CmdStreamHeader{
		Role: "es",
		DUID: deviceID,
	}

	err = protoj.StreamSendJSON(stream, header)
	if err != nil {
		return nil, err
	}

	wh := newHolder(deviceID, session, stream)
	return wh, nil
}

// cmdwsService long run service, never return
func cmdwsService() {
	// never return
	for {
		// build/re-build command websocket
		wh, err := buildCmdWS()
		if err != nil {
			log.Println("cmdwsService reconnect later, buildCmdWS failed:", err)
			time.Sleep(15 * time.Second)
			continue
		}

		wh.loop()
	}
}

// onPong update waiting response ping counter
func (wh *sessionholder) onPong(data []byte) {
	wh.waitingPingCount = 0
}

// keepalive send ping message peer, and counter
func (wh *sessionholder) keepalive() {
	if wh.stream == nil {
		return
	}

	// too many un-response ping, close the websocket connection
	if wh.waitingPingCount > 3 {
		if wh.sess != nil {
			log.Println("sessionholder keepalive failed, close:", wh.uuid)
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

// loop read command websocket and process command
func (wh *sessionholder) loop() {
	// save to map, for keep-alive
	holderMap[wh.uuid] = wh
	sess := wh.sess

	go wh.serveCmdStream()

	for {
		// TODO: accept streams
		stream, err := sess.AcceptStream(context.Background())
		if err != nil {
			log.Println("sess.AcceptStream failed:", err)
			break
		}

		// TODO: service link stream
		onPairRequest(stream)
	}

	// remove from map
	delete(holderMap, wh.uuid)
}

func (wh *sessionholder) serveCmdStream() {
	stream := wh.stream
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

// onPairRequest connect to local port via tcp,
// and then connect to server via websocket, bridge the two connections.
func onPairRequest(stream quic.Stream) {
	log.Println("onPairRequest, pair link stream")
	defer stream.Close()
	// TODO: read LinkStreamHeader
	message, err := protoj.StreamReadJSON(stream)
	if err != nil {
		log.Errorf("onPairRequest streamReadJSON failed:%v", err)
		return
	}

	var header = &protoj.LinkStreamHeader{}
	err = json.Unmarshal(message, header)
	if err != nil {
		log.Errorf("onPairRequest json.UnMarshal failed:%v", err)
		return
	}

	// target port
	port := header.Port

	// only allow connect to local host
	address := fmt.Sprintf("127.0.0.1:%d", port)

	log.Printf("onPairRequest, try link to:%s", address)

	// connect to local network via tcp
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Errorf("onPairRequest connect to address:%s failed:%v", address, err)
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
				log.Println("onPairRequest ws read error:", err)
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
			log.Println("onPairRequest tcp read error:", err)
			break
		}

		_, err = stream.Write(tcpbuf[:n])
		if err != nil {
			log.Println("onPairRequest ws write error:", err)
			break
		}
	}

	log.Println("onPairRequest, pair link stream end")
}
