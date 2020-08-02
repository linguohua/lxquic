package endpointc

import (
	"context"
	"fmt"
	"lxquic/protoj"
	"net"

	"github.com/lucas-clemente/quic-go"
	log "github.com/sirupsen/logrus"
)

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

		ssholder := getHolder(deviceID)
		if ssholder == nil {
			ssholder = buildQuicConnection("ec", deviceID)
		}

		if ssholder != nil {
			// Handle connections in a new goroutine.
			go handleRequest(conn.(*net.TCPConn), ssholder.sess)
		} else {
			conn.Close()
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
			err = protoj.WriteAll(conn, quicbuf[:n])
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
