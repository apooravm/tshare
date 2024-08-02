package receiver

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/apooravm/tshare-client/src/shared"
	"github.com/gorilla/websocket"
)

var (
	receiverPath     string
	unique_code      uint8
	count            = 1
	transferMD       MDReceiver
	totalArrivedSize int
	// Toggled to true when server notifies that its about to close the connection.
	CLOSE_CONN    = false
	IncomingFiles []shared.FileInfo
	// Keep track of file ids received
	FileIdsReceived []uint8

	// Increment with every new file being transfered
	ActiveTransferFileId    = 1
	activeFileBeingReceived *os.File

	progressBar *shared.ProgressBar
)

// Metadata for receiver from server
type MDReceiver struct {
	FileSize   uint64
	SenderName string
	Filename   string
}

func HandleReceiveArg(receiverName, targetDirPath, pbType string, pbRGBOn, pbIsMB bool, pbLength int, pbOff bool) error {
	receiverPath = targetDirPath

	var resUniqueCode string
	fmt.Println("Enter the code")
	fmt.Scan(&resUniqueCode)

	code, err := strconv.ParseUint(resUniqueCode, 10, 8)
	if err != nil {
		return fmt.Errorf("E:Could not parse input to uint8. Invalid input.")
	}

	unique_code = uint8(code)

	queryParams := url.Values{}
	queryParams.Add("intent", "receive")
	queryParams.Add("code", strconv.Itoa(int(code)))
	queryParams.Add("receivername", receiverName)

	finalURL := fmt.Sprintf("%s?%s", shared.Endpoint, queryParams.Encode())
	conn, err := shared.InitConnection(finalURL)
	if err != nil {
		return err
	}

	defer conn.Close()
	defer activeFileBeingReceived.Close()

	if err := HandleReceiverConn(conn, pbType, pbRGBOn, pbIsMB, pbLength, pbOff); err != nil {
		fmt.Println(err.Error())
	}

	return nil
}

func HandleReceiverConn(conn *websocket.Conn, pbType string, pbRGBOn, pbIsMB bool, pbLength int, pbOff bool) error {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if CLOSE_CONN {
				fmt.Println("Server closed the connection.")
				// No error is returned, graceful disconnect
				return nil
			}

			fmt.Println("Connection closed.")
			return err
		}

		switch message[1] {
		case shared.InitialTypeTextMessage:
			if len(message) > 2 {
				fmt.Printf("%s %s\n", shared.ColourSprintf("Server:", "cyan", false), string(message[2:]))
			}

		case shared.InitialTypeReceiverMD:
			if err := json.Unmarshal(message[2:], &IncomingFiles); err != nil {
				fmt.Println("Err umarshalling", err.Error())
				// Request disconn ig
			}

			if len(IncomingFiles) > 1 {
				shared.ColourPrint("Receiving files", "yellow")
			} else {

				shared.ColourPrint("Receiving file", "yellow")
			}

			totalFileSize := 0
			for _, file := range IncomingFiles {
				totalFileSize += int(file.Size)
				fmt.Printf("%d  %s - %s\n", file.Id, shared.ColourSprintf(fmt.Sprintf("[%.2fMB]", float64(file.Size)/float64(1000_000)), "yellow", false), file.RelativePath)
			}

			progressBar = shared.NewProgressBar(totalFileSize, pbType, pbLength, pbRGBOn, "", pbIsMB, pbOff)

			var resBeginTransfer string
			fmt.Println("Begin transfer? (y/n)")
			fmt.Scan(&resBeginTransfer)

			if resBeginTransfer == "yes" || resBeginTransfer == "y" || resBeginTransfer == "Y" {
				fmt.Println("Starting transfer")

				if err := CreateFileWithDirs(IncomingFiles[ActiveTransferFileId-1].RelativePath); err != nil {
					fmt.Println(err.Error())
					continue
				}

				// Start the transfer of a file with Id
				startTransferWithFileIdPkt, _ := shared.CreateBinaryPacket(shared.Version, shared.InitialTypeStartTransferWithId, uint8(ActiveTransferFileId))
				if err := conn.WriteMessage(websocket.BinaryMessage, startTransferWithFileIdPkt); err != nil {
					fmt.Println("E:Requesting next packet. Forcing disconnect.\n", err.Error())
					_ = conn.Close()
					return nil
				}

				progressBar.UpdateOngoingForNewFile(int(IncomingFiles[ActiveTransferFileId-1].Size))

			} else {
				// Abort transfer
				fmt.Println("Aborting transfer")
				abortpkt, _ := shared.CreateBinaryPacket(shared.Version, shared.InitialAbortTransfer)
				if err := conn.WriteMessage(websocket.BinaryMessage, abortpkt); err != nil {
					fmt.Println("E:Aborting transfer. Forcing disconnect.\n", err.Error())
					_ = conn.Close()
					return nil
				}
			}

		case shared.InitialTypeTransferPacket:
			if len(message) < 3 {
				fmt.Println("Empty file chunk received.")
				continue
			}

			incomingFileChunk := message[2:]
			// TODO: Add a connection close request here.

			_, err = activeFileBeingReceived.Write(incomingFileChunk)
			if err != nil {
				fmt.Println("E:Writing data.", err.Error())
				shared.RequestCloseConn(conn)
			}

			progressBar.UpdateTransferredSize(len(incomingFileChunk))
			progressBar.Show()

			// fmt.Print("\033[0K") // Clear the line from the cursor to the end
			nextPacketRequest, err := shared.CreateBinaryPacket(shared.Version, shared.InitialTypeRequestNextPacket)
			if err != nil {
				fmt.Println("\nCould not create next packet request.")
				continue
			}

			if err := conn.WriteMessage(websocket.BinaryMessage, nextPacketRequest); err != nil {
				fmt.Println("\nE:Writing file chunk.", err.Error())
				shared.RequestCloseConn(conn)
			}

		case shared.InitialTypeSingleFileTransferFinish:
			if err := activeFileBeingReceived.Close(); err != nil {
				fmt.Println("Could not close file.", IncomingFiles[ActiveTransferFileId-1].RelativePath, err.Error())
			}
			progressBar.PrintPostDoneMessage(fmt.Sprintf("Finished receiving file %s", IncomingFiles[ActiveTransferFileId-1].RelativePath))

			FileIdsReceived = append(FileIdsReceived, uint8(ActiveTransferFileId))

			// All files finished transferring
			if len(FileIdsReceived) == len(IncomingFiles) {
				continue
			}

			ActiveTransferFileId += 1

			if err := CreateFileWithDirs(IncomingFiles[ActiveTransferFileId-1].RelativePath); err != nil {
				fmt.Println(err.Error())
				continue
			}

			// Start the transfer of a file with Id
			startTransferWithFileIdPkt, _ := shared.CreateBinaryPacket(shared.Version, shared.InitialTypeStartTransferWithId, uint8(ActiveTransferFileId))
			if err := conn.WriteMessage(websocket.BinaryMessage, startTransferWithFileIdPkt); err != nil {
				fmt.Println("E:Requesting next packet. Forcing disconnect.\n", err.Error())
				_ = conn.Close()
				return nil
			}
			progressBar.UpdateOngoingForNewFile(int(IncomingFiles[ActiveTransferFileId-1].Size))

		case shared.InitialTypeAllTransferFinish:
			fmt.Println("\nAll files have been received.")

		case shared.InitialTypeCloseConnNotify:
			CLOSE_CONN = true
		}

	}
}

func CreateFileWithDirs(targetPath string) error {
	targetPath = filepath.Join(receiverPath, targetPath)
	var err error

	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
		return fmt.Errorf("Could not create dirs for incoming file. %s.\n%s", targetPath, err.Error())
	}

	activeFileBeingReceived, err = os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("Could not create incoming file. %s.\n%s", targetPath, err.Error())
	}

	return nil
}
