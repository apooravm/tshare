package receiver

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/apooravm/tshare-client/src/shared"
	"github.com/gorilla/websocket"
)

var (
	unique_code      uint8
	count            = 1
	transferMD       MDReceiver
	totalArrivedSize int
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
func ParsePacket(packetBytes []byte) ([]byte, error) {
	buffer := bytes.NewReader(packetBytes)

	var read_dataBytes = make([]byte, len(packetBytes))

	// incomingData := len(packetBytes[6:])
	if err := binary.Read(buffer, binary.BigEndian, &read_dataBytes); err != nil {
		return read_dataBytes, err
	}

	return read_dataBytes, nil
}

// Metadata for receiver from server
type MDReceiver struct {
	FileSize   uint64
	SenderName string
	Filename   string
}

// TODO: Instead of the conn being passed down here, create a func called GetConn or smn
// It attempts to connect to the server for no reason
func HandleReceiveArg(receiverName, targetDirPath string) error {
	var targetFile *os.File
	defer targetFile.Close()

	var resUniqueCode string
	fmt.Println("Enter the code")
	fmt.Scan(&resUniqueCode)

	code, err := strconv.ParseUint(resUniqueCode, 10, 8)
	if err != nil {
		return fmt.Errorf("E:Could not parse input to uint8. Invalid input.")
	}

	unique_code = uint8(code)
	pkt, err := CreateRegisterReceiverPkt(receiverName, uint8(code))
	if err != nil {
		return fmt.Errorf("E:Creating receiver register packet. %s", err.Error())
	}

	conn, err := shared.InitConnection()
	if err != nil {
		return fmt.Errorf("E:Could not connect. %s", err.Error())
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, pkt); err != nil {
		return fmt.Errorf("E:Sending receiver register packet. %s", err.Error())
	}

	// flag enabled after closeConn byte from server
	// For graceful exit
	connCloseFlag := false

	// Read loop
	for {
		_, response, err := conn.ReadMessage()
		if err != nil {
			if connCloseFlag {
				fmt.Println("\nServer closed the connection.")
				return nil
			}

			log.Println("E:Reading message.", err.Error())
			shared.RequestCloseConn(conn)
		}

		if len(response) == 0 {
			fmt.Println("E:Server responded with nothing.")
			shared.RequestCloseConn(conn)
		}

		// Handling initial byte type
		// [version][initial_byte][...]
		switch response[1] {
		// [version][unique_code][JsonMD]
		case shared.InitialTypeTransferMetaData:
			// Ignore the initial byte
			if err := json.Unmarshal(response[2:], &transferMD); err != nil {
				fmt.Println("Could not unmarshal", err.Error())
				shared.RequestCloseConn(conn)
			}

			// Join target dir and filename and create the file
			finalTargetFilePath := filepath.Join(targetDirPath, transferMD.Filename)
			targetFile, err = os.Create(finalTargetFilePath)
			if err != nil {
				return fmt.Errorf("E:Creating target file. %s", err.Error())
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
			resp, err := shared.CreateBinaryPacket(shared.Version, shared.InitialTypeRequestNextPkt, triggerByte)
			if err != nil {
				log.Println("E:creating binary response. Closing.", err.Error())
				shared.RequestCloseConn(conn)
			}

			if err := conn.WriteMessage(websocket.BinaryMessage, resp); err != nil {
				fmt.Println("E:Writing response. Closing.", err.Error())
				shared.RequestCloseConn(conn)
			}

		case shared.InitialTypeTransferPacket:
			count += 1
			incomingFileChunk, err := ParsePacket(response[2:])
			// TODO: Add a connection close request here.
			if err != nil {
				fmt.Println("E:Parsing incoming packet. Stopping.", err.Error())
				// _ = conn.Close()
			}

			_, err = targetFile.Write(incomingFileChunk)
			if err != nil {
				fmt.Println("E:Writing data.", err.Error())
				shared.RequestCloseConn(conn)
			}

			totalArrivedSize += len(response[2:])
			fmt.Printf("\r%d/%d %s", totalArrivedSize, transferMD.FileSize, "bytes")

			// fmt.Print("\033[0K") // Clear the line from the cursor to the end
			resp, err := shared.CreateBinaryPacket(shared.Version, shared.InitialTypeRequestNextPkt)
			if err != nil {
				fmt.Println("\nE:Creating file chunks.", err.Error())
				shared.RequestCloseConn(conn)
			}

			if err := conn.WriteMessage(websocket.BinaryMessage, resp); err != nil {
				fmt.Println("\nE:Writing file chunk.", err.Error())
				shared.RequestCloseConn(conn)
			}

		// Disconnection done by server
		case shared.InitialTypeFinishTransfer:
			fmt.Println("Transfer finished")

		// Text response from the server
		case shared.InitialTypeTextMessage:
			if len(response) > 2 {
				fmt.Println(string(response[2:]))
			}

		// Server sends this when it closes the connection from its side
		// Toggle flag to exit after crash
		case shared.InitialTypeCloseConn:
			connCloseFlag = true
			if len(response) == 3 {
				fmt.Println(string(response[2:]))
			}

		default:
			log.Println("Random initial byte", response[0])
			shared.RequestCloseConn(conn)
		}
	}

}

func ReceiveFile(conn *websocket.Conn) {
}

func VoluntaryDisconnect(conn *websocket.Conn) {

}

func CreateTargetFile() {
}
