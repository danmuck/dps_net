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

func (n *Node) StoreM(target NodeID, key string, value []byte) Message {
	return Message{
		Type:   Store,
		From:   n.Info,
		Target: target,
		Key:    key,
		Value:  value,
	}
}

func (n *Node) FindNodeM(target NodeID) Message {
	return Message{
		Type:   FindNode,
		From:   n.Info,
		Target: target,
	}
}

func (n *Node) FoundNodeM(target NodeID, results []Contact) Message {
	return Message{
		Type:    FoundNode,
		From:    n.Info,
		Target:  target,
		Results: results,
	}
}

func (n *Node) FindValueM(target NodeID, key string) Message {
	return Message{
		Type:   FindValue,
		From:   n.Info,
		Target: target,
		Key:    key,
	}
}

func (n *Node) FoundValueM(target NodeID, key string, value []byte, results []Contact) Message {
	return Message{
		Type:    FoundValue,
		From:    n.Info,
		Target:  target,
		Key:     key,
		Value:   value,
		Results: results,
	}
}
