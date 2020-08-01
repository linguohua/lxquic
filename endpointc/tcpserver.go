package endpointc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"lxquic/protoj"
	"net"

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

// startTCPListener start tcp server, listen on localhost
func startTCPListener(port uint16) {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("startTCPListener tcp server listen failed:", err)
	}

	for {
		// Listen for an incoming connection.
		conn, err := listener.Accept()
		if err != nil {
			log.Println("startTCPListener error accepting: ", err.Error())
			continue
		}

		ssholder := getHolder()
		if ssholder == nil {
			ssholder = buildQuicConnection()
		}

		if ssholder != nil {
			// Handle connections in a new goroutine.
			go handleRequest(conn.(*net.TCPConn), ssholder.sess)
		} else {
			conn.Close()
		}

	}
}

func getHolder() *sessionholder {
	h, _ := holderMap[deviceID]

	return h
}

func buildQuicConnection() *sessionholder {
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
		Role: "ec",
		DUID: deviceID,
		Port: int(remotePort),
	}

	err = protoj.StreamSendJSON(cmdStream, header)
	if err != nil {
		log.Println("handleRequest streamSendJSON failed:", err)
		return nil
	}

	var holder = newHolder(deviceID, session, cmdStream)
	holderMap[deviceID] = holder

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

// handleRequest read tcp connection, and send to server via websocket connection
func handleRequest(conn *net.TCPConn, session quic.Session) {
	log.Println("handleRequest new request")
	defer conn.Close()

	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		log.Println("handleRequest session.OpenStreamSync failed:", err)
		return
	}

	defer stream.Close()

	log.Println("handleRequest session.OpenStreamSync ok")
	quicbuf := make([]byte, 64*1024)
	// read websocket message and forward to tcp
	go func() {
		defer conn.Close()
		for {
			n, err := stream.Read(quicbuf)
			if err != nil {
				log.Println("handleRequest stream read error:", err)
				break
			}

			if n == 0 {
				break
			}

			// log.Println("handleRequest, ws message len:", len(message))
			err = writeAll(conn, quicbuf[:n])
			if err != nil {
				break
			}
		}
	}()

	// read tcp message and forward to websocket
	tcpbuf := make([]byte, 8192)
	for {
		n, err := conn.Read(tcpbuf)
		if err != nil {
			log.Println("handleRequest tcp read error:", err)
			break
		}

		_, err = stream.Write(tcpbuf[:n])
		if err != nil {
			log.Println("handleRequest ws write error:", err)
			break
		}
	}

	log.Println("handleRequest new request end")
}

// writeAll a function that ensure all bytes write out
// maybe it is unnecessary, if the underlying tcp connection has ensure that
func writeAll(conn *net.TCPConn, buf []byte) error {
	wrote := 0
	l := len(buf)
	for {
		n, err := conn.Write(buf[wrote:])
		if err != nil {
			return err
		}

		if n == 0 {
			// this should not happen
			break
		}

		wrote = wrote + n
		if wrote == l {
			break
		}
	}

	return nil
}
