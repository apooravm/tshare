package shared

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/websocket"
)

func CreateBinaryPacket(parts ...any) ([]byte, error) {
	responseBfr := new(bytes.Buffer)
	for _, part := range parts {
		if err := binary.Write(responseBfr, binary.BigEndian, part); err != nil {
			return nil, err
		}
	}

	return responseBfr.Bytes(), nil
}

// Returns the connected ws
func InitConnection(endpoint string) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
	if err != nil {
		log.Println("E:Connecting ws server.", err.Error())
		return nil, err
	}

	return conn, nil
}

func RequestCloseConn(conn *websocket.Conn) {
	packet, err := CreateBinaryPacket(Version, InitialTypeCloseConn)
	if err != nil {
		log.Println("E:Creating closure package. Quitting.")
		_ = conn.Close()
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, packet); err != nil {
		log.Println("E:Writing closure message to server. Quitting.")
		_ = conn.Close()
	}
}

// Can take in both a single file path or a path to some dir
// If dir is provided, all the files (even under other subdirs) are returned
func GetAllFileInfo(targetPath string) (*[]FileInfo, error) {
	var allFileInfo []FileInfo
	targetPathInfo, err := os.Stat(targetPath)
	if err != nil {
		return nil, fmt.Errorf("E:Getting provided path info. %s", err.Error())
	}

	var id_count uint8 = 1

	// Return single file with its name and size
	if !targetPathInfo.IsDir() {
		allFileInfo = append(allFileInfo, FileInfo{
			Name:         targetPathInfo.Name(),
			Size:         uint64(targetPathInfo.Size()),
			RelativePath: targetPathInfo.Name(),
			AbsPath:      targetPath,
			Id:           id_count,
		})

		return &allFileInfo, nil
	}

	folderName := filepath.Base(targetPath)

	if err := filepath.Walk(targetPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("E:Walking provided dir path. %s", err.Error())
		}

		// Making the paths of all the files relative to the target folder instead of caller.
		// Path parts of the file. Relative to caller not the target folder.
		path_parts := strings.Split(filepath.ToSlash(path), "/")
		splitIdx := 0
		for i := 0; i < len(path_parts); i++ {
			if path_parts[i] == folderName {
				splitIdx = i
				break
			}
		}

		absFilePath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("E:Could not get abs filepath. %s", err.Error())
		}

		pathRelativeToTargetFolder := strings.Join(path_parts[splitIdx:], "/")

		if !info.IsDir() {
			allFileInfo = append(allFileInfo, FileInfo{
				Name:         info.Name(),
				Size:         uint64(info.Size()),
				AbsPath:      absFilePath,
				RelativePath: pathRelativeToTargetFolder,
				Id:           id_count,
			})

			id_count += 1
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &allFileInfo, nil
}

// Get coloured string
func ColourSprintf(message string, colour string, endL bool) string {
	finalChar := ""
	if endL {
		finalChar = "\n"
	}

	switch colour {
	case "red":
		return fmt.Sprintf("\x1b[31m%s\x1b[0m%s", message, finalChar)

	case "yellow":
		return fmt.Sprintf("\x1b[33m%s\x1b[0m%s", message, finalChar)

	case "green":
		return fmt.Sprintf("\x1b[32m%s\x1b[0m%s", message, finalChar)

	case "magenta":
		return fmt.Sprintf("\x1b[35m%s\x1b[0m%s", message, finalChar)

	case "cyan":
		return fmt.Sprintf("\x1b[36m%s\x1b[0m%s", message, finalChar)

	case "blue":
		return fmt.Sprintf("\x1b[34m%s\x1b[0m%s", message, finalChar)

	default:
		return fmt.Sprintf("%s%s", message, finalChar)
	}
}

func ColourLogPrint(colour string, endL bool, message ...string) {
	joinedMessage := strings.Join(message, "")
	log.Println(ColourSprintf(joinedMessage, colour, endL))
}

func ColourPrint(message string, colour string) {
	fmt.Println(ColourSprintf(message, colour, false))
}
