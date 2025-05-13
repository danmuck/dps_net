package dht

type MessageType string

const (
	Ping       MessageType = "PING"
	Pong       MessageType = "PONG"
	Store      MessageType = "STORE"
	FindNode   MessageType = "FIND_NODE"
	FoundNode  MessageType = "FOUND_NODE"
	FindValue  MessageType = "FIND_VALUE"
	FoundValue MessageType = "FOUND_VALUE"
)

type Message struct {
	Type    MessageType `json:"type"`
	From    Contact     `json:"from"`
	Target  NodeID      `json:"target,omitempty"`
	Key     string      `json:"key,omitempty"`
	Value   []byte      `json:"value,omitempty"`
	Results []Contact   `json:"results,omitempty"`
}

func (n *Node) PingM(target NodeID) Message {
	return Message{
		Type:   Ping,
		From:   n.Info,
		Target: target,
	}
}

func (n *Node) PongM(target NodeID) Message {
	return Message{
		Type:   Pong,
		From:   n.Info,
		Target: target,
	}
}
