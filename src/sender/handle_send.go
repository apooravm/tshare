package sender

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/apooravm/tshare-client/src/shared"
	"github.com/gorilla/websocket"
)

var (
	unique_code  uint8
	fileToBeSent *os.File
	chunkSize    uint32
	sendBuf      []byte
	sendCount    = 1
	// Toggled to true when server notifies that its about to close the connection.
	CLOSE_CONN          = false
	activeFileIdx       uint8
	activeFileBeingSent *os.File
	filesBeingSent      *[]shared.FileInfo
	fileDataPacket      []byte
	// Keep track of file ids sent
	FileIdsSent       []uint8
	CurrFileBeingSent *shared.FileInfo
	TotalFilesSize    int
	CurrFileSentSize  int
	TotalFileSentSize int

	allFilesTotalSize  int
	allFilesSentSize   int
	allFilesSingleBlip int

	currFileSentSize   int
	currFileSingleBlip int

	transferStarted bool

	progressBar       *shared.ProgressBar
	currTransferSpeed float64
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

func HandleSendArg(chunk_size uint32, filesize int64, senderName string, allFileInfo *[]shared.FileInfo, pbType string, pbRGBOn, pbIsMB bool, pbLength int, pbOff bool) error {
	paramQuery := url.Values{}
	filesBeingSent = allFileInfo
	chunkSize = chunk_size

	totalFileSize := 0

	for _, info := range *allFileInfo {
		totalFileSize += int(info.Size)
		infoValue := fmt.Sprintf("%s,%s,%d", info.RelativePath, strconv.Itoa(int(info.Size)), info.Id)
		paramQuery.Add("fileinfo", infoValue)
	}

	progressBar = shared.NewProgressBar(totalFileSize, pbType, pbLength, pbRGBOn, "", pbIsMB, pbOff)

	paramQuery.Add("intent", "send")
	paramQuery.Add("sendername", senderName)

	finalURL := fmt.Sprintf("%s?%s", shared.Endpoint, paramQuery.Encode())

	conn, err := shared.InitConnection(finalURL)
	if err != nil {
		return err
	}

	if len(*allFileInfo) > 1 {
		shared.ColourPrint("Sending files", "yellow")
	} else {

		shared.ColourPrint("Sending file", "yellow")
	}

	for _, file := range *allFileInfo {
		fmt.Printf("%d  %s - %s\n", file.Id, shared.ColourSprintf(fmt.Sprintf("[%.2fMB]", float64(file.Size)/float64(1000_000)), "yellow", false), file.RelativePath)
	}

	defer conn.Close()
	if err := HandleSenderConn(conn); err != nil {
		fmt.Println(err.Error())
	}

	return nil
}

func HandleSenderConn(conn *websocket.Conn) error {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if CLOSE_CONN {
				fmt.Println("Server closed the connection.")
				return nil
			}

			fmt.Println("Connection closed.")
			return err
		}

		switch message[1] {
		case shared.InitialTypeTransferCode:
			if len(message) < 3 {
				// Idk request disconnection or smn
			}

			unique_code = message[2]
			fmt.Println("Transfer code is", unique_code)

		// TODO: If id not found, reply ...
		case shared.InitialTypeStartTransferWithId:
			var fileId uint8 = message[2]
			fileFound := false
			for _, beingSentFile := range *filesBeingSent {
				if beingSentFile.Id == fileId {
					CurrFileBeingSent = &beingSentFile
					file, err := os.Open(beingSentFile.AbsPath)
					if err != nil {
						// Abort operation?
						fmt.Println("Err opening file...")
						continue
					}

					progressBar.UpdateOngoingForNewFile(int(beingSentFile.Size))
					currFileSingleBlip = int(beingSentFile.Size) / 20
					sendBuf = make([]byte, chunkSize)
					activeFileBeingSent = file
					fileFound = true
				}
			}

			if !fileFound {
				fmt.Println("File not found, id", fileId)

			} else {
				if err := SendNextPacket(conn); err != nil {
					fmt.Println("Could not send file chunk.", err.Error())
					continue
				}
			}

		case shared.InitialTypeRequestNextPacket:
			if err := SendNextPacket(conn); err != nil {
				fmt.Println("Could not send file chunk.", err.Error())
				continue
			}

		case shared.InitialTypeTextMessage:
			if len(message) > 2 {
				fmt.Printf("%s %s\n", shared.ColourSprintf("Server:", "cyan", false), string(message[2:]))
			}

		// Only used to toggle this flag, which doesnt throw error when conn is closed.
		case shared.InitialTypeCloseConnNotify:
			CLOSE_CONN = true
		}

	}
}

func SendNextPacket(conn *websocket.Conn) error {
	fileBytes, isEOF, err := GetNextFileBytes()
	if err != nil {
		// Abort?
		fmt.Println("E:Reading file.", err.Error())
	}

	if isEOF {
		progressBar.PrintPostDoneMessage(fmt.Sprintf("Finished uploading file %s", CurrFileBeingSent.RelativePath))
		FileIdsSent = append(FileIdsSent, CurrFileBeingSent.Id)
		transferStarted = false

		currFileTransferDonePkt, _ := shared.CreateBinaryPacket(shared.Version, shared.InitialTypeSingleFileTransferFinish)
		if err := conn.WriteMessage(websocket.BinaryMessage, currFileTransferDonePkt); err != nil {
			fmt.Println("E:Sending single file transfer finish ping. Forcing disconnect.\n", err.Error())
			_ = conn.Close()
			return err
		}

		// If all files transferred
		if len(FileIdsSent) == len(*filesBeingSent) {
			fmt.Println()
			allFilesTransferPkt, _ := shared.CreateBinaryPacket(shared.Version, shared.InitialTypeAllTransferFinish)
			if err := conn.WriteMessage(websocket.BinaryMessage, allFilesTransferPkt); err != nil {
				fmt.Println("E:Sending single file transfer finish ping. Forcing disconnect.\n", err.Error())
				_ = conn.Close()
				return err
			}
		}

		return nil
	}

	progressBar.UpdateTransferredSize(len(fileBytes))
	progressBar.Show()

	// Refer to shared.Packet
	// [Version 1byte][Init_byte 1byte][timestamp int64][datachunk...]
	fileDataPacket, err = shared.CreateBinaryPacket(shared.Version, shared.InitialTypeTransferPacket, time.Now().UnixMilli(), fileBytes)
	if err != nil {
		fmt.Println("Could not create filebytes packet...")
		return nil
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, fileDataPacket); err != nil {
		fmt.Println("E:Sending file packet. Forcing disconnect.\n", err.Error())
		_ = conn.Close()
		return err
	}

	return nil
}

func GetNextFileBytes() ([]byte, bool, error) {
	// Reads len(buf) -> 1024 bytes and stores them into buf itself
	n, err := activeFileBeingSent.Read(sendBuf)
	if err != nil {
		if err == io.EOF {
			return nil, true, nil
		}

		return nil, false, err

	}

	// EOF
	if n == 0 {
		return nil, true, nil
	}

	return sendBuf[:n], false, nil
}
