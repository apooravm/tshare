package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/gorilla/websocket"
)

const (
	MessageTypeRegister = 0x01
	// For the actual transfer
	MessageTypeTransfer = 0x02
	version             = byte(1)
)

// Packet breakdown
// [message_type 1byte][version 1byte][unique_code 1byte][data_size 2byte][sender/receiver][sender/receiver name][remaining for the data]

// Since handshake is a 1 time thing, it will be done through json
type ClientHandshake struct {
	Version uint8
	// Sender => 0, Receiver => 1
	Intent     uint8
	UniqueCode uint8
	FileSize   uint16
	ClientName string
	Filename   string
}

type Packet struct {
	Version    byte
	UniqueCode byte
	DataSize   uint16
	Data       []byte
}

// Every client needs to register with the server by handshaking with ClientHandshake obj
// It is difficult to differentiate between register requests and packet transfer requests
// Thus a special 1 byte is appended to the start of each request to indicate the type.
func CreateRegisterRequestPacket(handshakeObj *ClientHandshake) ([]byte, error) {
	messageType := []byte{MessageTypeRegister}
	requestJson, err := json.Marshal(handshakeObj)
	if err != nil {
		return nil, err
	}

	message := append(messageType, requestJson...)
	return message, nil
}

// []byte type in go is already a reference type
func SerializePacket(outgoingPacket *Packet) ([]byte, error) {
	buffer := new(bytes.Buffer)

	// Append an indicator byte to tell the server that this is for transfer
	// 2 variants for register and transfer
	if err := binary.Write(buffer, binary.BigEndian, MessageTypeTransfer); err != nil {
		return nil, err
	}

	if err := binary.Write(buffer, binary.BigEndian, outgoingPacket.Version); err != nil {
		return nil, err
	}

	if err := binary.Write(buffer, binary.BigEndian, outgoingPacket.UniqueCode); err != nil {
		return nil, err
	}

	if err := binary.Write(buffer, binary.BigEndian, outgoingPacket.DataSize); err != nil {
		return nil, err
	}

	if err := binary.Write(buffer, binary.BigEndian, outgoingPacket.Data); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// Parse the incoming []byte into a Packet object.
// Converting to struct might be an overhead.
// Maybe should try a more direct method.
// TBD
func ParsePacket(packetBytes []byte) (*Packet, error) {
	var receivedPacket Packet
	buffer := bytes.NewReader(packetBytes)

	// 1 byte, version
	if err := binary.Read(buffer, binary.BigEndian, &receivedPacket.Version); err != nil {
		return &receivedPacket, err
	}

	// 1 byte/uint8, unique_code
	if err := binary.Read(buffer, binary.BigEndian, &receivedPacket.UniqueCode); err != nil {
		return &receivedPacket, err
	}

	// 4 byte/uint8, data_size
	if err := binary.Read(buffer, binary.BigEndian, &receivedPacket.DataSize); err != nil {
		return &receivedPacket, err
	}

	// Initialize a size of the Data, read into it normally
	receivedPacket.Data = make([]byte, receivedPacket.DataSize)
	if err := binary.Read(buffer, binary.BigEndian, &receivedPacket.Data); err != nil {
		return &receivedPacket, err
	}

	return &receivedPacket, nil
}

// TODO: Implement cli using flags
func main() {
	cli_args := os.Args[1:]
	if len(cli_args) == 0 {
		fmt.Println("Insufficient Arguments.\n'tshare receive | send'")
		return
	}

	// wsUrl := "ws://localhost:4000/api/fileshare"
	//
	// conn, _, err := websocket.DefaultDialer.Dial(wsUrl, nil)
	// if err != nil {
	// 	log.Println("E:Connecting ws server.", err.Error())
	// 	return
	// }
	//
	// defer conn.Close()

	senderName := "mrSender"
	// receiverName := "mrReceiver"

	// Maybe get input
	choice := cli_args[0]
	if choice == "send" {
		// Send metadata
		// Filename, filesize, sender name
		// TODO: Receive back some random generated code, used for receiver auth

		filepath := "./local/sample_vid.mp4"

		fileinfo, err := os.Stat(filepath)
		if err != nil {
			log.Println("E:Getting fileinfo.", err.Error())
			return
		}

		if fileinfo.IsDir() {
			log.Println("Must be a file.")
			return
		}

		fmt.Printf("Sending %s [%.2fMB]\n", fileinfo.Name(), float64(fileinfo.Size())/float64(1000_000))

		pkt, err := CreateRegisterRequestPacket(&ClientHandshake{
			Version:    version,
			Intent:     0,
			UniqueCode: 0,
			ClientName: senderName,
			FileSize:   uint16(fileinfo.Size()),
			Filename:   fileinfo.Name(),
		})
		if err != nil {
			log.Println("E:Creating handshake packet.", err.Error())
			return
		}

		fmt.Printf("% X \n", pkt)

		// SendFile(filepath, conn)

	} else if choice == "receive" {
		// ReceiveFile(conn)

	} else {
		fmt.Println("Invalid Argument \n'tshare-client.exe send <path> | receive'")

	}

}

func ReceiveFile(conn *websocket.Conn) {
	filepath := "./local/received/sample_vid.mp4"

	targetFile, err := os.Create(filepath)
	if err != nil {
		log.Println("E:Creating target file.", err.Error())
		return
	}

	defer targetFile.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("E:Reading chunk.", err.Error())
			return
		}

		_, err = targetFile.Write(message)
		if err != nil {
			log.Println("E:Writing data.", err.Error())
			return
		}
	}
}

func SendFile(filepath string, conn *websocket.Conn) {
	file, err := os.Open(filepath)
	if err != nil {
		log.Println("E:Opening file.", err.Error())
		return
	}

	defer file.Close()

	buf := make([]byte, 1024)
	for {
		// Reads len(buf) -> 1024 bytes and stores them into buf itself
		n, err := file.Read(buf)
		if err != nil {
			log.Println("E:Reading file.", err.Error())
			return
		}

		// EOF
		if n == 0 {
			break
		}

		// buf[:n] to send only the valid portion of read data
		// So if at the end only 9 bytes were read, we send only that 9 byte slice.
		if err = conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
			log.Println("E:Sending chunk.", err.Error())
			return
		}
	}
}
