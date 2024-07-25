package sender

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/apooravm/tshare-client/src/shared"
	"github.com/gorilla/websocket"
)

var (
	unique_code uint8
)

// Since handshake is a 1 time thing, it will be done through json
type ClientHandshake struct {
	Version uint8
	// Sender => 0, Receiver => 1
	Intent     uint8
	UniqueCode uint8
	FileSize   uint64
	ClientName string
	Filename   string
}

// Send metadata
// Filename, filesize, sender name
// TODO: Receive back some random generated code, used for receiver auth
func HandleSendArg(conn *websocket.Conn) {
	senderName := "mrSender"
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

	pkt, err := CreateRegisterSenderPkt(&ClientHandshake{
		Version:    shared.Version,
		Intent:     0,
		UniqueCode: 0,
		ClientName: senderName,
		FileSize:   uint64(fileinfo.Size()),
		Filename:   fileinfo.Name(),
	})
	if err != nil {
		log.Println("E:Creating handshake packet.", err.Error())
		return
	}

	// fmt.Printf("% X \n", pkt)
	if err := conn.WriteMessage(websocket.BinaryMessage, pkt); err != nil {
		log.Println("E:Writing message.", err.Error())
		return
	}

	// Read loop
	for {
		_, response, err := conn.ReadMessage()
		if err != nil {
			log.Println("E:Reading message.", err.Error())
			return
		}

		if len(response) == 0 {
			log.Println("E:Server responded with nothing.")
			conn.Close()
			return
		}

		// first byte will always be the version
		// second is the initial_byte
		// [version][initial_byte][...]
		// Handling initial byte type
		switch response[1] {
		// [version][initial_byte][unique_code]
		case shared.InitialTypeUniqueCode:
			if len(response) < 3 {
				log.Println("E:No code provided. Closing")
				conn.Close()
				return
			}

			unique_code = uint8(response[2])
			fmt.Printf("Transfer code is %d. Waiting for the receiver...\n", unique_code)

		// Text response from the server
		case shared.InitialTypeTextMessage:
			if len(response) > 2 {
				fmt.Println(string(response[2:]))

			}

		// response[1] -> Begin transfer
		// reponse[1] -> Abrort transfer
		// [versioj][initial_byte][beginTransferOrNo]
		case shared.InitialTypeBeginTransfer:
			if len(response) < 3 {
				log.Println("E:Server responded with nothing.")
				_ = conn.Close()
				return
			}

			var beginTransferOrNo uint8 = response[2]
			if beginTransferOrNo == 1 {
				fmt.Println("Starting file transfer")
				SendFile(filepath, conn)

			} else if beginTransferOrNo == 0 {
				fmt.Println("Receiver has aborted the file transfer.")
			}
		}
	}
}

// Every client needs to register with the server by handshaking with ClientHandshake obj
// It is difficult to differentiate between register requests and packet transfer requests
// Thus a special 1 byte is appended to the start of each request to indicate the type.
func CreateRegisterSenderPkt(handshakeObj *ClientHandshake) ([]byte, error) {
	messageType := []byte{shared.Version, shared.InitialTypeRegisterSender}
	requestJson, err := json.Marshal(handshakeObj)
	if err != nil {
		return nil, err
	}

	message := append(messageType, requestJson...)
	return message, nil
}

// []byte type in go is already a reference type
func SerializePacket(outgoingPacket *shared.Packet) ([]byte, error) {
	buffer := new(bytes.Buffer)

	// Append an indicator byte to tell the server that this is for transfer
	// 2 variants for register and transfer
	if err := binary.Write(buffer, binary.BigEndian, shared.InitialTypeTransferPacket); err != nil {
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

// Refactor to send only after ack
func SendFile(filepath string, conn *websocket.Conn) {
	file, err := os.Open(filepath)
	if err != nil {
		log.Println("E:Opening file.", err.Error())
		return
	}

	defer file.Close()

	buf := make([]byte, 16384)
	for {
		// Reads len(buf) -> 1024 bytes and stores them into buf itself
		n, err := file.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Println("E:Reading file.", err.Error())
			} else {
				fmt.Println("Finished upload")
			}
			return
		}

		// EOF
		if n == 0 {
			conn.Close()
			break
		}

		pkt_frame, err := SerializePacket(&shared.Packet{
			Version:    shared.Version,
			UniqueCode: unique_code,
			DataSize:   uint16(n),
			Data:       buf[:n],
		})

		if err != nil {
			fmt.Println("E:Could not SerializePacket.", err.Error())
			_ = conn.Close()
			return
		}

		// buf[:n] to send only the valid portion of read data
		// So if at the end only 9 bytes were read, we send only that 9 byte slice.
		if err = conn.WriteMessage(websocket.BinaryMessage, pkt_frame); err != nil {
			log.Println("E:Sending chunk.", err.Error())
			return
		}
	}
}
