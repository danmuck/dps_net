package api

import (
	"time"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

// NewContact builds a *api.Contact from its raw fields.
func NewContact(id []byte, address, tcpPort, udpPort string) *Contact {
	return &Contact{
		Id:         id,
		UdpAddress: address + ":" + udpPort,
		TcpAddress: address + ":" + tcpPort,
		LastSeen:   timestamppb.New(time.Now()),
	}
}

func (c *Contact) ID() NodeID {
	return NodeID(c.GetId())
}
