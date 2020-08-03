package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"lxquic/protoj"
	"math/big"
	"time"

	"encoding/json"
	"encoding/pem"

	"github.com/lucas-clemente/quic-go"
	log "github.com/sirupsen/logrus"
)

var (
	ecIndex    = 0
	ecmap      = make(map[int]*ecEndpoint)
	esmap      = make(map[string]*esEndpoint)
	pxmap      = make(map[int]*pxEndpoint)
	proxyToken string
)

// keepalive send ping to all websocket
func keepalive() {
	for {
		time.Sleep(time.Second * 10)

		// first keepalive all xport/web-ssh websocket
		for _, v := range esmap {
			v.keepalive()
		}

		for _, v := range pxmap {
			v.keepalive()
		}

		// then keepalive pair websocket
		for _, v := range ecmap {
			v.keepalive()
		}
	}
}

// Params parameters
type Params struct {
	// server listen address
	ListenAddr string

	ProxyToken string
}

// CreateQuicServer start http server
func CreateQuicServer(params *Params) {
	// start keepalive goroutine
	go keepalive()

	proxyToken = params.ProxyToken

	log.Printf("quic server listen at:%s", params.ListenAddr)

	listener, err := quic.ListenAddr(params.ListenAddr, generateTLSConfig(), nil)
	if err != nil {
		log.Fatalln("quic.ListenAddr failed:", err)
	}

	for {
		sess, err := listener.Accept(context.Background())
		if err != nil {
			log.Fatalln("listener.Accept failed:", err)
		}

		go onAcceptSession(sess)
	}
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-echo-example"},
	}
}

func onAcceptSession(sess quic.Session) {
	log.Println("onAcceptSession quic server accept a new session")
	defer func() {
		log.Println("quic server onAcceptSession exit")
		sess.CloseWithError(0, "out of scope")
	}()

	stream, err := sess.AcceptStream(context.Background())
	if err != nil {
		log.Println("onAcceptSession sess.AcceptStream failed:", err)
		return
	}

	message, err := protoj.StreamReadJSON(stream)
	if err != nil {
		log.Errorf("onAcceptSession streamReadJSON failed:%v", err)
		return
	}

	var h = &protoj.CmdStreamHeader{}
	err = json.Unmarshal(message, h)
	if err != nil {
		log.Errorf("onAcceptSession json.Unmarshal failed:%v", err)
		return
	}

	switch h.Role {
	case "es":
		serveES(sess, stream, h)
		break
	case "ec":
		serveEC(sess, stream, h)
		break
	case "px":
		servePX(sess, stream, h)
		break
	}
}
