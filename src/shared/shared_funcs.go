package shared

import (
	"bytes"
	"encoding/binary"
	"log"

	"github.com/gorilla/websocket"
)

func CreateBinaryPacket(parts ...any) ([]byte, error) {
	responseBfr := new(bytes.Buffer)
	for _, part := range parts {
		if err := binary.Write(responseBfr, binary.BigEndian, part); err != nil {
			return nil, err
		}
	}

	return responseBfr.Bytes(), nil
}

// Returns the connected ws
func InitConnection() (*websocket.Conn, error) {
	wsUrl := "ws://localhost:4000/api/share"
	wsUrl = "wss://multi-serve.onrender.com/api/share"

	conn, _, err := websocket.DefaultDialer.Dial(wsUrl, nil)
	if err != nil {
		log.Println("E:Connecting ws server.", err.Error())
		return nil, err
	}

	return conn, nil
}

func RequestCloseConn(conn *websocket.Conn) {
	packet, err := CreateBinaryPacket(Version, InitialTypeCloseConn)
	if err != nil {
		log.Println("E:Creating closure package. Quitting.")
		_ = conn.Close()
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, packet); err != nil {
		log.Println("E:Writing closure message to server. Quitting.")
		_ = conn.Close()
	}
}
