package main

import (
	"fmt"
	"log"
	"os"

	"github.com/apooravm/tshare-client/src/receiver"
	"github.com/apooravm/tshare-client/src/sender"
	"github.com/gorilla/websocket"
)

// Packet breakdown
// [initial_byte][version][unique_code][data_in_pkt_size][data]

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
		sender.HandleSendArg(conn)

	} else if choice == "receive" {
		receiver.HandleReceiveArg(conn)

	} else {
		fmt.Println("Invalid Argument \n'tshare-client.exe send <path> | receive'")

	}

}
