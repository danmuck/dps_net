package dht

// func (n *Node) SendMessageTo(addrStr string, msg Message) {
// 	udpAddr, err := net.ResolveUDPAddr("udp", addrStr)
// 	if err != nil {
// 		LogRPCFailure("Invalid address: " + err.Error())
// 		return
// 	}

// 	conn, err := net.DialUDP("udp", nil, udpAddr)
// 	if err != nil {
// 		LogRPCFailure("Dial error: " + err.Error())
// 		return
// 	}
// 	defer conn.Close()

// 	msg.From = Contact{ID: n.ID, Address: n.Addr}
// 	n.Conn.SendMessage(udpAddr.String(), msg)
// }
