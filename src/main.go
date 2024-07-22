package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"os"
)

// Packet breakdown
// [version 1byte][unique_code 1byte][data_size 2byte][sender/receiver][sender/receiver name][remaining for the data]

type Packet struct {
	Version          byte
	UniqueCode       byte
	DataSize         uint16
	SenderOrReceiver byte
	ClientName       string
	Data             []byte
}

// Endianness refers to byte order in a multi-byte object

// TODO: Implement cli using flags
func main() {
	cli_args := os.Args[1:]
	if len(cli_args) == 0 {
		fmt.Println("Insufficient Arguments.\n'tshare receive | send'")
		return
	}

	wsUrl := "ws://localhost:4000/api/fileshare"

	conn, _, err := websocket.DefaultDialer.Dial(wsUrl, nil)
	if err != nil {
		log.Println("E:Connecting ws server.", err.Error())
		return
	}

	defer conn.Close()

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

		fmt.Printf("Sending %s [%.2fMB]", fileinfo.Name(), float64(fileinfo.Size())/float64(1000_000))

		SendFile(filepath, conn)

	} else if choice == "receive" {
		ReceiveFile(conn)

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
