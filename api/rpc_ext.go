package api

import (
	"time"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

type FILE RPC

// NewContact builds a *api.Contact from its raw fields.
func NewContact(id []byte, address, tcpPort, udpPort string) *Contact {
	return &Contact{
		Id:       id,
		Username: "default",
		Address:  address,
		UdpPort:  udpPort,
		TcpPort:  tcpPort,
		LastSeen: timestamppb.New(time.Now()),
	}
}

func (c *Contact) UpdateUsername(name string) error {
	c.Username = name
	return nil
}

func (c *Contact) GetUDPAddress() string {
	return c.Address + ":" + c.UdpPort
}

func (c *Contact) GetTCPAddress() string {
	return c.Address + ":" + c.TcpPort
}

func (c *Contact) ID() NodeID {
	return NodeID(c.GetId())
}
