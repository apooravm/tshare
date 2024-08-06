package shared

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
	InitialTypeCloseConn = uint8(0x08)

	// Text message about some issue or error or whatever
	// InitialTypeTextMessage = uint8(0x09)

	InitialTypeRequestNextPkt = uint8(0x10)
	InitialTypeFinishTransfer = uint8(0x11)

	// Server responds with transfer metadata of the transfer to receiver
	InitialTypeReceiverMD = uint8(0x21)

	// Receiver requests next pkt from server which inturn requests from sender
	InitialTypeRequestNextPacket = uint8(0x22)

	// A single file has finished transferring
	InitialTypeSingleFileTransferFinish = uint8(0x23)

	// All files have finished transferring
	InitialTypeAllTransferFinish = uint8(0x24)

	// Client, sender or receiver, requests the server to abort the transfer.
	InitialTypeAbortTransfer = uint8(0x25)

	// Server messages client
	InitialTypeTextMessage = uint8(0x26)

	// Server notifies the client that the connection is going to be closed.
	InitialTypeCloseConnNotify = uint8(0x27)

	// Server responds back to sender with the transfer code
	InitialTypeTransferCode = uint8(0x28)

	// Receiver aborts the transfer
	InitialAbortTransfer = uint8(0x29)

	// Receiver invokes a transfer of file with given idx from the server.
	InitialTypeStartTransferWithId = uint8(0x30)

	// current version
	Version = byte(1)
)

var (
	Endpoint = "wss://multi-serve.onrender.com/api/share"
)

type Packet struct {
	Version    byte
	UniqueCode byte
	Timestamp  int64
	Data       []byte
}

type FileInfo struct {
	Name string
	// Relative to the target folder.
	RelativePath string
	// Abs path of the file in the system.
	AbsPath string
	Size    uint64

	// Unique id for each file. Used to invoke transfer of a certain file from the sender.
	// Only 254 files can be shared at once
	Id uint8
}
