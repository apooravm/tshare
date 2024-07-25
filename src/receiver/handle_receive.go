package receiver

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/apooravm/tshare-client/src/shared"
	"github.com/gorilla/websocket"
)

var (
	unique_code uint8
)

// Receiver packet
// [version][initial_byte][unique_code][receiver_name]
func CreateRegisterReceiverPkt(receiverName string, uniqueCode uint8) ([]byte, error) {
	handshakeBuffer := new(bytes.Buffer)

	// Version
	if err := binary.Write(handshakeBuffer, binary.BigEndian, shared.Version); err != nil {
		return nil, err
	}

	// Initial byte
	if err := binary.Write(handshakeBuffer, binary.BigEndian, shared.InitialTypeRegisterReceiver); err != nil {
		return nil, err
	}

	// Unique code
	if err := binary.Write(handshakeBuffer, binary.BigEndian, uniqueCode); err != nil {
		return nil, err
	}

	// Receiver name
	if err := binary.Write(handshakeBuffer, binary.BigEndian, []byte(receiverName)); err != nil {
		return nil, err
	}

	return handshakeBuffer.Bytes(), nil
}

// Parse the incoming []byte into a Packet object.
// Converting to struct might be an overhead.
// Maybe should try a more direct method.
// TBD
func ParsePacket(packetBytes []byte) (*shared.Packet, error) {
	var receivedPacket shared.Packet
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

// Metadata for receiver from server
type MDReceiver struct {
	FileSize   uint64
	SenderName string
	Filename   string
}

// TODO: Instead of the conn being passed down here, create a func called GetConn or smn
// It attempts to connect to the server for no reason
func HandleReceiveArg(conn *websocket.Conn) {
	receiverName := "mrReceiver"

	var resUniqueCode string
	fmt.Println("Enter the code")
	fmt.Scan(&resUniqueCode)

	code, err := strconv.ParseUint(resUniqueCode, 10, 8)
	if err != nil {
		log.Println("E:Could not parse input to uint8. Invalid input.")
		return
	}

	unique_code = uint8(code)
	pkt, err := CreateRegisterReceiverPkt(receiverName, uint8(code))
	if err != nil {
		log.Println("E:Creating receiver register packet.", err.Error())
		return
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, pkt); err != nil {
		log.Println("E:Sending receiver register packet.", err.Error())
		return
	}

	filepath := "./local/received/sample_vid.mp4"

	targetFile, err := os.Create(filepath)
	if err != nil {
		log.Println("E:Creating target file.", err.Error())
		return
	}

	defer targetFile.Close()

	// flag enabled after closeConn byte from server
	// For graceful exit
	connCloseFlag := true

	// Read loop
	for {
		_, response, err := conn.ReadMessage()
		if err != nil {
			if connCloseFlag {
				fmt.Println("Server closed the connection.")
				return
			}

			log.Println("E:Reading message.", err.Error())
			return
		}

		if len(response) == 0 {
			log.Println("E:Server responded with nothing.")
			conn.Close()
			return
		}

		// Handling initial byte type
		// [version][initial_byte][...]
		switch response[1] {
		// [version][unique_code][JsonMD]
		case shared.InitialTypeTransferMetaData:
			var transferMD MDReceiver
			// Ignore the initial byte
			if err := json.Unmarshal(response[2:], &transferMD); err != nil {
				fmt.Println("Could not unmarshal", err.Error())
				_ = conn.Close()
				return
			}

			fmt.Printf("Receiving %s [%.2fMB] from %s\n", transferMD.Filename, float64(transferMD.FileSize)/float64(1000_000), transferMD.SenderName)

			var res string
			fmt.Println("Begin transfer? (y/n)")
			fmt.Scan(&res)

			var triggerByte uint8
			if res == "yes" || res == "y" || res == "Y" {
				triggerByte = 1
				fmt.Println("Starting transfer")
			} else {
				// Abort transfer
				fmt.Println("Aborting transfer")
				triggerByte = 0
			}

			// Begin transfer
			// Begintransfer from receiver packet frame
			// [initial_byte][trigger_byte][unique_code]
			resp, err := shared.CreateBinaryPacket(shared.Version, shared.InitialTypeBeginTransfer, triggerByte)
			if err != nil {
				log.Println("E:creating binary response. Closing.", err.Error())
				_ = conn.Close()
				return
			}

			if err := conn.WriteMessage(websocket.BinaryMessage, resp); err != nil {
				fmt.Println("E:Writing response. Closing.", err.Error())
				_ = conn.Close()
				return
			}

		case shared.InitialTypeTransferPacket:
			pkt, err := ParsePacket(response[1:])
			// TODO: Add a connection close request here.
			if err != nil {
				log.Println("E:Parsing incoming packet. Stopping.", err.Error())
				_ = conn.Close()
			}

			// _, message, err := conn.ReadMessage()
			// if err != nil {
			// 	log.Println("E:Reading chunk.", err.Error())
			// 	return
			// }

			// fmt.Println(pkt.DataSize)
			_, err = targetFile.Write(pkt.Data)
			if err != nil {
				log.Println("E:Writing data.", err.Error())
				return
			}

		// Text response from the server
		case shared.InitialTypeTextMessage:
			if len(response) > 2 {
				fmt.Println(string(response[2:]))
			}

		// Server sends this when it closes the connection from its side
		// Toggle flag to exit after crash
		case shared.InitialTypeCloseConn:
			fmt.Println("Anyhi")
			if len(response) == 3 {
				fmt.Println(string(response[2:]))
			}

		default:
			log.Println("Random initial byte", response[0])
			return
		}
	}

}

func ReceiveFile(conn *websocket.Conn) {
}

func VoluntaryDisconnect(conn *websocket.Conn) {

}
