package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	"github.com/gorilla/websocket"
)

const (
	// Register a sender
	InitialTypeRegisterSender = uint8(0x01)

	// Server responds back to sender with a unique code
	InitialTypeUniqueCode = uint8(0x02)

	// Register a receiver
	InitialTypeRegisterReceiver = uint8(0x03)

	// Server sends metadata of the transfer to the receiver
	InitialTypeTransferMetaData = uint8(0x04)

	// Server responds back to sender to begin transfer
	// Receiver responds with 1 or 0
	// 1 to begin transfer. 0 to abort.
	InitialTypeBeginTransfer = uint8(0x05)

	// Transfer packet from sender to receiver.
	InitialTypeTransferPacket = uint8(0x06)

	// Volutanry disconnection
	InitialTypeCloseConn = uint(0x08)

	// Text message about some issue or error or whatever
	InitialTypeTextMessage = uint8(0x09)

	// current version
	version = byte(1)
)

var (
	unique_code uint8
)

// Packet breakdown
// [message_type 1byte][version 1byte][unique_code 1byte][data_size 2byte][sender/receiver][sender/receiver name][remaining for the data]

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

type Packet struct {
	Version    byte
	UniqueCode byte
	DataSize   uint16
	Data       []byte
}

// Every client needs to register with the server by handshaking with ClientHandshake obj
// It is difficult to differentiate between register requests and packet transfer requests
// Thus a special 1 byte is appended to the start of each request to indicate the type.
func CreateRegisterSenderPkt(handshakeObj *ClientHandshake) ([]byte, error) {
	messageType := []byte{InitialTypeRegisterSender}
	requestJson, err := json.Marshal(handshakeObj)
	if err != nil {
		return nil, err
	}

	message := append(messageType, requestJson...)
	return message, nil
}

// Receiver packet
// [initial_byte][unique_code][receiver_name]
func CreateRegisterReceiverPkt(receiverName string, uniqueCode uint8) ([]byte, error) {
	handshakeBuffer := new(bytes.Buffer)

	// Initial byte
	if err := binary.Write(handshakeBuffer, binary.BigEndian, InitialTypeRegisterReceiver); err != nil {
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

// []byte type in go is already a reference type
func SerializePacket(outgoingPacket *Packet) ([]byte, error) {
	buffer := new(bytes.Buffer)

	// Append an indicator byte to tell the server that this is for transfer
	// 2 variants for register and transfer
	if err := binary.Write(buffer, binary.BigEndian, InitialTypeTransferPacket); err != nil {
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

	wsUrl := "ws://localhost:4000/api/share"

	conn, _, err := websocket.DefaultDialer.Dial(wsUrl, nil)
	if err != nil {
		log.Println("E:Connecting ws server.", err.Error())
		return
	}

	defer conn.Close()

	// Maybe get input
	choice := cli_args[0]
	if choice == "send" {
		HandleSendArg(conn)

	} else if choice == "receive" {
		// ReceiveFile(conn)
		HandleReceiveArg(conn)

	} else {
		fmt.Println("Invalid Argument \n'tshare-client.exe send <path> | receive'")

	}

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
		Version:    version,
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

		// Handling initial byte type
		switch response[0] {
		case InitialTypeUniqueCode:
			if len(response) < 2 {
				log.Println("E:No code provided. Closing")
				conn.Close()
				return
			}

			unique_code = uint8(response[1])
			fmt.Printf("Transfer code is %d. Waiting for the receiver...\n", unique_code)

		// Text response from the server
		case InitialTypeTextMessage:
			if len(response) > 1 {
				fmt.Println("Server:", string(response[1:]))

			}

		// response[1] -> Begin transfer
		// reponse[1] -> Abrort transfer
		case InitialTypeBeginTransfer:
			if len(response) != 2 {
				log.Println("E:Server responded with nothing.")
				_ = conn.Close()
				return
			}

			var beginTransferOrNo uint8 = response[1]
			if beginTransferOrNo == 1 {
				fmt.Println("Starting file transfer")
				SendFile(filepath, conn)

			} else if beginTransferOrNo == 0 {
				fmt.Println("Aborting file transfer")
			}
		}
	}

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

		// Handling initial byte type
		switch response[0] {
		case InitialTypeTransferMetaData:
			var transferMD MDReceiver
			// Ignore the initial byte
			if err := json.Unmarshal(response[1:], &transferMD); err != nil {
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
			var resBytes bytes.Buffer

			if err := binary.Write(&resBytes, binary.BigEndian, InitialTypeBeginTransfer); err != nil {
				fmt.Println("E:Creating binary response. Closing.", err.Error())
				_ = conn.Close()
				return
			}

			// 1 triggers transfer
			if err := binary.Write(&resBytes, binary.BigEndian, triggerByte); err != nil {
				fmt.Println("E:Creating binary response. Closing.", err.Error())
				_ = conn.Close()
				return
			}

			if err := conn.WriteMessage(websocket.BinaryMessage, resBytes.Bytes()); err != nil {
				fmt.Println("E:Writing response. Closing.", err.Error())
				_ = conn.Close()
				return
			}

		case InitialTypeTransferPacket:
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
		case InitialTypeTextMessage:
			if len(response) > 1 {
				fmt.Println("Server:", string(response[1:]))

			}

		default:
			log.Println("Random initial byte", response[0])
			return
		}
	}

}

func ReceiveFile(conn *websocket.Conn) {
}

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

		pkt_frame, err := SerializePacket(&Packet{
			Version:    version,
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
