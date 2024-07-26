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
	unique_code  uint8
	fileToBeSent *os.File
	// chunkSize    uint8 = 128
	// chunkSize uint16 = 2048
	// chunk uint16 = 16384
	chunkSize uint32 = 65536
	sendBuf          = make([]byte, chunkSize)
	sendCount        = 1
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
	// filepath = "./local/sample.txt"

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

	connCloseFlag := false

	fileToBeSent, err = os.Open(filepath)
	if err != nil {
		log.Println("E:Opening file.", err.Error())
		return
	}

	defer fileToBeSent.Close()

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
				// SendFile(filepath, conn)

			} else if beginTransferOrNo == 0 {
				fmt.Println("Receiver has aborted the file transfer.")
			}

		case shared.InitialTypeRequestNextPkt:
			pkt, isEOF, err := RequestNextPkt()

			if err != nil {
				fmt.Println("E:Getting next packet")
				// conn.Close()
			}

			if isEOF {
				resp, err := shared.CreateBinaryPacket(shared.Version, shared.InitialTypeFinishTransfer)
				if err != nil {
					fmt.Println("E:Creating packet. EOF reached. Finishing transfer", err.Error())
				}
				if err := conn.WriteMessage(websocket.BinaryMessage, resp); err != nil {
					fmt.Println(err.Error())
				}
			}

			finalpkt, err := shared.CreateBinaryPacket(shared.Version, shared.InitialTypeTransferPacket)
			finalpkt = append(finalpkt, pkt...)

			// buf[:n] to send only the valid portion of read data
			// So if at the end only 9 bytes were read, we send only that 9 byte slice.
			sendCount += 1
			if err = conn.WriteMessage(websocket.BinaryMessage, finalpkt); err != nil {
				log.Println("E:Sending chunk.", err.Error())

			}

		case shared.InitialTypeCloseConn:
			connCloseFlag = true
			if len(response) == 3 {
				fmt.Println(string(response[2:]))
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
func SerializePacket(dataPacket []byte) ([]byte, error) {
	buffer := new(bytes.Buffer)

	if err := binary.Write(buffer, binary.BigEndian, shared.Version); err != nil {
		return nil, err
	}

	// Append an indicator byte to tell the server that this is for transfer
	// 2 variants for register and transfer
	if err := binary.Write(buffer, binary.BigEndian, shared.InitialTypeTransferPacket); err != nil {
		return nil, err
	}

	if err := binary.Write(buffer, binary.BigEndian, dataPacket); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func SendNextPacket() {}

func InitFileForTransfer() {}

// Returns the next packet from the file
// If EOF, returns false
func RequestNextPkt() ([]byte, bool, error) {
	// Reads len(buf) -> 1024 bytes and stores them into buf itself
	n, err := fileToBeSent.Read(sendBuf)
	if err != nil {
		if err == io.EOF {
			fmt.Println("Finished upload")
			return nil, true, nil
		}

		log.Println("E:Reading file.", err.Error())
		return nil, false, err

	}

	// EOF
	if n == 0 {
		return nil, true, nil
	}

	// packet_frame, err := SerializePacket(sendBuf[:n])
	// if err != nil {
	// 	fmt.Println("E:Could not SerializePacket.", err.Error())
	// 	return nil, false, nil
	// }

	return sendBuf[:n], false, nil
}
