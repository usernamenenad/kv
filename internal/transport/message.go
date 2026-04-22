package transport

type MessageType uint8

const (
	VoteRequest MessageType = iota
	VoteResponse
	LogRequest
	LogResponse
)

type Message struct {
	Type   MessageType
	Sender uint64
	Data   any
}
