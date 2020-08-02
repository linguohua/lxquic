package endpointc

import (
	"context"
	"fmt"
	"io"
	"lxquic/endpointc/socks5"
	"lxquic/protoj"
	"net"

	"github.com/lucas-clemente/quic-go"

	log "github.com/sirupsen/logrus"
)

type socksReqHandler struct {
}

func (sh *socksReqHandler) HandleRequest(req *socks5.SocksRequest) error {
	ssholder := getHolder(proxyToken)
	if ssholder == nil {
		ssholder = buildQuicConnection("px", proxyToken)
	}

	if ssholder != nil {
		// Handle connections in a new goroutine.
		handleSocks5Request(req, ssholder.sess)
	} else {
		return fmt.Errorf("no quic session avaible, discard socks request")
	}

	return nil
}

func startSocks5Server() {
	var sh = &socksReqHandler{}
	config := &socks5.Config{ReqHandler: sh}
	s, err := socks5.New(config)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("socks server listen at:%d", socks5Port)
	log.Fatal(s.ListenAndServe("tcp", fmt.Sprintf("127.0.0.1:%d", socks5Port)))
}

func handleSocks5Request(req *socks5.SocksRequest, sess quic.Session) {
	log.Println("handleSocks5Request")
	conn := req.Conn.(*net.TCPConn)
	defer conn.Close()

	// create link stream
	stream, err := sess.OpenStreamSync(context.Background())
	if err != nil {
		log.Println("handleSocks5Request session.OpenStreamSync failed:", err)
		return
	}

	defer stream.Close()

	address := req.DestAddr
	var host string
	var port = address.Port
	if address.FQDN != "" {
		host = address.FQDN
	} else {
		host = address.IP.String()
	}

	// send link header
	var header = &protoj.LinkStreamHeader{
		Port: port,
		Host: host,
	}

	log.Printf("handleSocks5Request target host:%s, target port:%d", host, port)
	protoj.StreamSendJSON(stream, header)
	if err != nil {
		log.Printf("handleSocks5Request StreamSendJSON failed:%v, discard", err)
		return
	}

	quicbuf := make([]byte, 64*1024)
	// read websocket message and forward to tcp
	go func() {
		// read tcp message and forward to websocket
		tcpbuf := make([]byte, 8192)
		for {
			n, err := conn.Read(tcpbuf)
			//log.Printf("handleSocks5Request tcp read bytes:%d, err:%v", n, err)
			if err != nil {
				log.Println("handleSocks5Request tcp read error:", err)
				if err != io.EOF {
					stream.Close()
				}
				break
			}

			if n == 0 {
				break
			}

			_, err = stream.Write(tcpbuf[:n])
			if err != nil {
				log.Println("handleSocks5Request ws write error:", err)
				break
			}
		}
	}()

	for {
		n, err := stream.Read(quicbuf)
		if err != nil {
			log.Println("handleSocks5Request stream read error:", err)
			break
		}

		if n == 0 {
			break
		}

		// log.Println("handleSocks5Request, ws message len:", len(message))
		err = protoj.WriteAll(conn, quicbuf[:n])
		if err != nil {
			break
		}
	}

	log.Println("handleSocks5Request new request end")
}
