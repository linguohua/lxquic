package protoj

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net"

	quic "github.com/lucas-clemente/quic-go"
)

// LinkStreamHeader link stream first packet
type LinkStreamHeader struct {
	Port int    `json:"port"`
	Host string `json:"host,omitempty"`
}

// CmdStreamHeader cmd stream first packet
type CmdStreamHeader struct {
	Role string `json:"role"`
	DUID string `json:"duid"`
	Port int    `json:"port,omitempty"`
}

// StreamCmd command
type StreamCmd struct {
	Cmd string `json:"cmd"`
}

// StreamReadJSON read json buffer
func StreamReadJSON(stream quic.Stream) ([]byte, error) {
	var lenb [2]byte
	_, err := io.ReadFull(stream, lenb[0:])
	if err != nil {
		return nil, err
	}

	len := binary.LittleEndian.Uint16(lenb[0:2])
	var bb = make([]byte, len)
	_, err = io.ReadFull(stream, bb[0:])
	if err != nil {
		return nil, err
	}

	return bb, nil
}

// StreamSendJSON send json object
func StreamSendJSON(stream quic.Stream, j interface{}) error {
	message, err := json.Marshal(j)
	if err != nil {
		return err
	}

	var lenb [2]byte
	binary.LittleEndian.PutUint16(lenb[0:], uint16(len(message)))

	// register via cmd stream
	_, err = stream.Write(lenb[0:2])
	if err != nil {
		return err
	}

	_, err = stream.Write(message)
	if err != nil {
		return err
	}

	return nil
}

// WriteAll a function that ensure all bytes write out
// maybe it is unnecessary, if the underlying tcp connection has ensure that
func WriteAll(conn net.Conn, buf []byte) error {
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
