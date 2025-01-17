package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/apooravm/tshare-client/src/receiver"
	"github.com/apooravm/tshare-client/src/sender"
	"github.com/apooravm/tshare-client/src/shared"
)

var (
	receivePath = "./received"
	pbType      = "total"
	pbRGBOn     = false
	pbLength    = 20
	pbIsMB      = true
	pbOff       = false
	client_name string
	// chunkSize   uint32 = 262144
	chunkSize uint32 = 1000 * 1024
	// chunkSize uint32 = 128
	// chunkSize    uint8 = 128
	// chunkSize uint16 = 2048
	// chunk uint16 = 16384
	// chunkSize uint32 = 65536
	// chunkSize uint32 = 262144
)

func main() {
	cli_args := os.Args[1:]
	if len(cli_args) == 0 {
		fmt.Println("Insufficient Arguments.\n'tshare receive | send'")
		return
	}

	handleArgs()
}

func handleArgs() {
	if len(os.Args) <= 1 {
		fmt.Println("Invalid Argument \nTry 'tshare-client.exe send <path> | receive | help'")
		return
	}

	if err := handleFlags(); err != nil {
		fmt.Println(err.Error())
		return
	}

	command := os.Args[1]
	switch command {
	case "send":
		if len(os.Args) <= 2 {
			fmt.Println("No file provided. 'tshare-client.exe send <path/to/file>'")
			return
		}

		targetPath := os.Args[2]

		fileinfo, err := os.Stat(targetPath)
		if err != nil {
			log.Println("E:Getting fileinfo.", err.Error())
			return
		}

		if client_name == "" {
			client_name = "Sender"
		}

		allFileInfo, err := shared.GetAllFileInfo(targetPath)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		if len(*allFileInfo) == 0 {
			fmt.Println("Target empty.")
			return
		}

		if len(*allFileInfo) == 1 {
			fmt.Printf("Sending %s [%.2fMB]. %d bytes per packet.\n", fileinfo.Name(), float64(fileinfo.Size())/float64(1000_000), chunkSize)
		}

		sender.HandleSendArg(uint32(chunkSize), fileinfo.Size(), client_name, allFileInfo, pbType, pbRGBOn, pbIsMB, pbLength, pbOff)

	case "receive":
		if len(os.Args) > 2 && os.Args[2][0] != '-' {
			receivePath = os.Args[2]
		}

		handleFolderCreate()

		fileinfo, err := os.Stat(receivePath)
		if err != nil {
			fmt.Println("E:Getting pathinfo.", err.Error())
			return
		}

		if !fileinfo.IsDir() {
			fmt.Println("E:Target must be a folder.")
			return
		}

		if client_name == "" {
			client_name = "Receiver"
		}

		receiver.HandleReceiveArg(client_name, receivePath, pbType, pbRGBOn, pbIsMB, pbLength, pbOff)

	case "help":
		PrintHelp()

	default:
		fmt.Println("Invalid Argument \nTry 'tshare-client.exe send <path> | receive | help'")
		return
	}
}

func PrintHelp() {
	fmt.Println("App usage: 'tshare-client.exe [COMMAND] [CMD_ARG] -[SUB_CMD]=[SUB_CMD_ARG]'")
	fmt.Println("\nCommands -")
	fmt.Println("Send - Send a file. Point to any file. 'tshare-client.exe send <path/to/file>'")
	fmt.Println("Receive - Receive a file. Custom target folder can be assigned by passing it next. 'tshare-client.exe receive [CUST_RECV_PATH]")
	fmt.Println("Help - Display this helper text. 'tshare-client.exe help")
	fmt.Println("\nSubcommands - Attach these at the end")
	fmt.Println("Set a custom chunk size. '-chunk=<CHUNK_SIZE>'")
	fmt.Println("Set a custom chunk multiple. chunkSize -> (x * 1024) '-chunkm=<NUM>'")
	fmt.Println("Set a custom client name. '-name=<NAME>'")
	fmt.Println("Set to dev mode. '-mode=dev'")
	fmt.Println("Set progress bar type. all/single '-pbtype=single'")
	fmt.Println("Set progress bar length. Default is 20. '-pblen=50'")
	fmt.Println("Set progress bar rgb colouring. rgb/normal '-pbcolour=rgb'")
	fmt.Println("Set progress bar size unit. mb/kb '-pbunit=mb'")
	fmt.Println("Turn off the progress bar. '-pb=off'")
}

// Not really needed anymore
func handleFolderCreate() {
	// Check if the folder exists
	if _, err := os.Stat(receivePath); os.IsNotExist(err) {
		// Create the folder if it doesn't exist
		err := os.Mkdir(receivePath, 0755)
		if err != nil {
			fmt.Println("Error creating folder:", err)
			return
		}
	} else {
	}
}

// Handle flags
func handleFlags() error {
	for _, arg := range os.Args[1:] {
		if arg[0] != '-' {
			continue
		}

		// arg[1:] to remove the -
		argParts := strings.Split(arg[1:], "=")
		if len(argParts) != 2 {
			return fmt.Errorf("Invalid flag format.")
		}

		switch argParts[0] {
		case "chunk":
			cSize, err := strconv.ParseUint(argParts[1], 10, 32)
			if err != nil {
				return fmt.Errorf("Invalid chunk size")
			}

			chunkSize = uint32(cSize)

		case "chunkm":
			cSize, err := strconv.ParseUint(argParts[1], 10, 32)
			if err != nil {
				return fmt.Errorf("Invalid chunk multiple. Must be uint32.")
			}

			chunkSize = uint32(cSize) * 1024

		case "name":
			client_name = argParts[1]

		// Settint to devmode
		case "mode":
			if argParts[1] == "dev" {
				shared.Endpoint = "ws://localhost:4000/api/share"
			}

		case "pbtype":
			switch argParts[1] {
			case "total":
				pbType = "total"
			case "single":
				pbType = "single"
			default:
				return fmt.Errorf("Invalid progress bar type. Must be total/single.")
			}

		case "pblen":
			pbLen, err := strconv.ParseInt(argParts[1], 10, 16)
			if err != nil {
				return fmt.Errorf("Invalid progress bar length size.")
			}

			pbLength = int(pbLen)

		case "pbcolour":
			switch argParts[1] {
			case "normal":
			case "rgb":
				pbRGBOn = true
			default:
				return fmt.Errorf("Invalid progress bar colour type. Must be normal/rgb.")
			}

		case "pbunit":
			switch argParts[1] {
			case "kb":
				pbIsMB = false
			case "mb":
				pbIsMB = true
			default:
				return fmt.Errorf("Invalid progress bar size unit. Must be kb/mb.")
			}

		case "pb":
			if argParts[1] != "off" {
				return fmt.Errorf("Invalid progress bar arg. Must be off.")
			} else {
				pbOff = true
			}

		default:
			fmt.Println("Invalid flag")
		}
	}

	return nil
}
