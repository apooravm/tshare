package sender

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strconv"

	"github.com/apooravm/tshare-client/src/shared"
	"github.com/gorilla/websocket"
)

var (
	unique_code  uint8
	fileToBeSent *os.File
	// sendBuf          = make([]byte, chunkSize)
	sendBuf   []byte
	sendCount = 1
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

func HandleSendArg(chunkSize uint32, filesize int64, senderName string, allFileInfo *[]shared.FileInfo) error {
	paramQuery := url.Values{}

	for _, info := range *allFileInfo {
		infoValue := fmt.Sprintf("%s,%s", info.RelativePath, strconv.Itoa(int(info.Size)))
		paramQuery.Add("fileinfo", infoValue)
	}

	paramQuery.Add("intent", "send")
	paramQuery.Add("sendername", senderName)

	finalURL := fmt.Sprintf("%s?%s", shared.Endpoint, paramQuery.Encode())

	conn, err := shared.InitConnection(finalURL)
	if err != nil {
		return err
	}

	defer conn.Close()

	return nil
}

// Send metadata
// Filename, filesize, sender name
// TODO: Receive back some random generated code, used for receiver auth
func HandleConn(chunkSize uint32, fileSize int64, senderName, filepath, fileName string) error {
	sendBuf = make([]byte, chunkSize)

	pkt, err := CreateRegisterSenderPkt(&ClientHandshake{
		Version:    shared.Version,
		Intent:     0,
		UniqueCode: 0,
		ClientName: senderName,
		FileSize:   uint64(fileSize),
		Filename:   fileName,
	})
	if err != nil {
		return fmt.Errorf("E:Creating handshake packet. %s", err.Error())
	}

	conn, err := shared.InitConnection(shared.Endpoint)
	if err != nil {
		return fmt.Errorf("E:Could not connect. %s", err.Error())
	}

	defer conn.Close()

	return nil
	// fmt.Printf("% X \n", pkt)
	if err := conn.WriteMessage(websocket.BinaryMessage, pkt); err != nil {
		return fmt.Errorf("E:Writing to server. %s", err.Error())
	}

	connCloseFlag := false

	fileToBeSent, err = os.Open(filepath)
	if err != nil {
		return fmt.Errorf("E:Opening file. %s", err.Error())
	}

	defer fileToBeSent.Close()

	// Read loop
	for {
		_, response, err := conn.ReadMessage()
		if err != nil {
			if connCloseFlag {
				fmt.Println("Server closed the connection.")
			}

			return fmt.Errorf("E:Reading server response. %s", err.Error())
		}

		if len(response) == 0 {
			log.Println("Server responded with nothing.")
			shared.RequestCloseConn(conn)
		}

		// first byte will always be the version
		// second is the initial_byte
		// [version][initial_byte][...]
		// Handling initial byte type
		switch response[1] {
		// [version][initial_byte][unique_code]
		case shared.InitialTypeUniqueCode:
			if len(response) < 3 {
				log.Println("Server responded with no code.")
				shared.RequestCloseConn(conn)
			}

			unique_code = uint8(response[2])
			fmt.Printf("Transfer code is %d. Waiting for the receiver...\n", unique_code)

		// Text response from the server
		case shared.InitialTypeTextMessage:
			if len(response) > 2 {
				fmt.Println(string(response[2:]))

			}

		case shared.InitialTypeRequestNextPkt:
			pkt, isEOF, err := RequestNextPkt()

			if err != nil {
				fmt.Println("E:Getting next packet")
				shared.RequestCloseConn(conn)
				// conn.Close()
			}

			if isEOF {
				resp, err := shared.CreateBinaryPacket(shared.Version, shared.InitialTypeFinishTransfer)
				if err != nil {
					fmt.Println("E:Creating packet. EOF reached. Finishing transfer", err.Error())
					shared.RequestCloseConn(conn)
				}

				if err := conn.WriteMessage(websocket.BinaryMessage, resp); err != nil {
					fmt.Println("E:Writing to server.", err.Error())
					shared.RequestCloseConn(conn)
				}
			}

			finalpkt, err := shared.CreateBinaryPacket(shared.Version, shared.InitialTypeTransferPacket)
			finalpkt = append(finalpkt, pkt...)

			// buf[:n] to send only the valid portion of read data
			// So if at the end only 9 bytes were read, we send only that 9 byte slice.
			sendCount += 1
			if err = conn.WriteMessage(websocket.BinaryMessage, finalpkt); err != nil {
				log.Println("E:Sending chunk.", err.Error())
				shared.RequestCloseConn(conn)

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
