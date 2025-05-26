package network

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/danmuck/dps_net/api"
	"github.com/danmuck/dps_net/config"
	"github.com/danmuck/dps_net/network/routing"
	"github.com/danmuck/dps_net/network/services"
	"github.com/danmuck/dps_net/network/transport"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// ////
// Generic handler function type for handling service/plugin RPCs
// Implemented on a per service basis
// //
type RPC_Handler func(ctx context.Context, payload []byte) ([]byte, error)

// ////
// Network Manager handles all network traffic
// //
type NetworkManager struct {
	nodeID    api.NodeID                    // local nodeID
	localAddr string                        // local network address <ip:port>
	router    *services.KademliaServiceImpl // p2p network routing table
	info      *api.Contact

	receiver chan transport.Packet
	udpServ  transport.Server // UDP packet server
	tcpServ  transport.Server // TCP packet server

	peers              map[string]*api.Contact                // username -> contact
	active             map[*api.Contact]bool                  // contact -> isActive
	appLocks           map[string]api.AppLock                 // appID -> appLock
	appHandlerRegistry map[api.AppLock]map[string]RPC_Handler // name -> rpc name -> handler

	lock sync.RWMutex
}

// ////
// Initialize a new NetworkManager for a Node
// //
func NewNetworkManager(local *api.Contact, cfg config.Config) (*NetworkManager, error) {

	// initialize local Contact info
	// local := api.NewContact(id[:], cfg.Address, tcpPort, udpPort)
	nm := &NetworkManager{
		nodeID:    local.ID(),
		localAddr: cfg.Address,
		router: services.NewKademliaService(
			routing.NewRoutingTable(local, cfg.K, cfg.Alpha),
		),
		info: local,

		receiver: nil,
		udpServ:  nil,
		tcpServ:  nil,

		peers:              make(map[string]*api.Contact),
		active:             make(map[*api.Contact]bool),
		appLocks:           make(map[string]api.AppLock),
		appHandlerRegistry: make(map[api.AppLock]map[string]RPC_Handler),
	}

	// add known apps by Lock
	// TODO: needs hardening
	for svc, str := range cfg.AppLocks {
		// convert config locks to raw bytes
		b, err := hex.DecodeString(str)
		if err != nil {
			return nil, fmt.Errorf("[NetworkManager] invalid app lock for %q: %w", svc, err)
		}
		// verify length -> 64 bytes / 128 chars as string for sha512 hash
		if len(b) != api.ApplicationIDBytes {
			return nil, fmt.Errorf("[NetworkManager] app lock for %q has wrong length: got %d, want %d",
				svc, len(b), api.ApplicationIDBytes)
		}
		// initialize an api.AppLock and add it to the registry
		var app_lock api.AppLock
		copy(app_lock[:], b)
		nm.appLocks[svc] = app_lock
	}

	kad := services.KademliaService_ServiceDesc
	kadDesc := &grpc.ServiceDesc{
		ServiceName: kad.ServiceName,
		HandlerType: kad.HandlerType,
		Methods:     kad.Methods,
		Streams:     kad.Streams,
		Metadata:    api.AppLockToSlice(nm.appLocks[kad.ServiceName]),
	}
	err := nm.RegisterService(kadDesc, nm.router)
	if err != nil {
		return nil, err
	}

	log.Printf("[NewNetworkManager] local=%s udp_port=%d",
		cfg.Address, cfg.UDPPort)

	func() {
		var appLockLog strings.Builder
		appLockLog.WriteString("[NetworkManager] \n -- AppLock Registry  -- \n")
		for k, v := range nm.appLocks {
			entry := fmt.Sprintf("  Service: %v \n  -> appLock: %x \n", k, v)
			appLockLog.WriteString(entry)
			// log.Printf("Service: %v \n -> appLock: %v \n", k, v)
		}
		appLockLog.WriteString(" -- -- -- -- -- -- -- -- ")
		log.Println(appLockLog.String())
	}()

	return nm, nil
}

func (nm *NetworkManager) Start() error {
	log.Printf("[NetworkManager] starting \n")
	nm.receiver = make(chan transport.Packet)
	usrv, err := transport.NewUDPServer(nm.localAddr, nm.info.UdpPort, nm.receiver)
	if err != nil {
		return err
	}
	nm.udpServ = usrv
	if err = nm.udpServ.Start(); err != nil {
		return err
	}

	if nm.udpServ != nil || nm.tcpServ != nil {
		go nm.serve()
	} else {
		return fmt.Errorf("neither udp or tcp are ready to serve")
	}
	return nil
}

func (nm *NetworkManager) Shutdown() {
	log.Printf("[NetworkManager] shutting down")
	if nm.udpServ != nil {
		nm.udpServ.Stop()
	}
	if nm.tcpServ != nil {
		nm.tcpServ.Stop()
	}
	if nm.receiver != nil {
		close(nm.receiver)
	}
	return
}

// ////
// RegisterService introspects a gRPC ServiceDesc and registers each method via reflection.
// //
func (m *NetworkManager) RegisterService(svcDesc *grpc.ServiceDesc, service any) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// TODO: deal with checking appLock and verifying
	appLock, ok := svcDesc.Metadata.([]byte)
	if !ok || appLock == nil || len(appLock) != api.ApplicationIDBytes {
		return fmt.Errorf("service %q missing hash metadata: %v", svcDesc.ServiceName, appLock)
	}
	expected, ok := m.appLocks[svcDesc.ServiceName]
	if !ok || bytes.Compare(appLock, expected[:]) != 0 {
		return fmt.Errorf("no trusted hash for service %q", svcDesc.ServiceName)
	}

	log.Printf("[NetworkManager] registering %q with lock %x", svcDesc.ServiceName, appLock)

	// initialize the handlerRegistry entry for this service
	m.appHandlerRegistry[api.AppLock(appLock)] = make(map[string]RPC_Handler)
	// reflect the service type and begin registering its methods
	svcType := reflect.TypeOf(service)
	for _, md := range svcDesc.Methods {
		log.Printf("           method %q", md.MethodName)

		method, ok := svcType.MethodByName(md.MethodName)
		if !ok {
			// skip if svcType does not match
			continue
		}

		// initialize handler for each service method
		handler := func(ctx context.Context, payload []byte) ([]byte, error) {
			// decode the request embedded in payload
			reqType := method.Type.In(2)
			reqVal := reflect.New(reqType.Elem())
			if err := proto.Unmarshal(payload, reqVal.Interface().(proto.Message)); err != nil {
				return nil, fmt.Errorf("unmarshal %s.%s request: %w", svcDesc.ServiceName, md.MethodName, err)
			}

			// call method: func(ctx, *Req) (*Res, error)
			results := method.Func.Call([]reflect.Value{
				reflect.ValueOf(service),
				reflect.ValueOf(ctx),
				reqVal,
			})

			// handle error if present
			errVal := results[1]
			if !errVal.IsNil() {
				return nil, errVal.Interface().(error)
			}

			// marshal response and return it
			resMsg := results[0].Interface().(proto.Message)
			resBytes, err := proto.Marshal(resMsg)
			if err != nil {
				return nil, fmt.Errorf("marshal %s.%s response: %w", svcDesc.ServiceName, md.MethodName, err)
			}
			return resBytes, nil
		}

		// add each handler to the registry
		m.appHandlerRegistry[api.AppLock(appLock)][md.MethodName] = handler
	}

	return nil
}

// ////
// Serves UDP packets
// //
func (m *NetworkManager) serve() {
	log.Printf("[NetworkManager] serving")
	for pkt := range m.receiver {
		go func(pkt transport.Packet) {
			m.lock.RLock()
			defer m.lock.RUnlock()
			log.Printf("[NetworkManager]:%v packet received for %v", m.info.Username, pkt.Sender)

			appLock := m.appLocks[pkt.RPC.Service]
			// delegate lookup & call to DispatchRPC
			log.Printf("[NetworkManager]:%v Dispatching RPC %s.%s",
				m.info.Username, pkt.RPC.Service, pkt.RPC.Method)
			respPayload, err := m.DispatchRPC(pkt.Ctx, appLock, pkt.RPC.Method, pkt.RPC.Payload)
			if err != nil {
				log.Printf("[NetworkManager] handler %s.%s error: %v",
					pkt.RPC.Service, pkt.RPC.Method, err)
				return
			}

			log.Printf("[NetworkManager] %s.%s → reply %d bytes",
				pkt.RPC.Service, pkt.RPC.Method, len(respPayload))

			replyEnvelope := &api.RPC{
				Service: pkt.RPC.Service,
				Method:  pkt.RPC.Method,
				Sender:  m.info,
				Payload: respPayload,
			}

			// mark peer active
			m.peers[pkt.Sender.Username] = pkt.Sender
			m.active[pkt.Sender] = true

			if err := pkt.Reply(replyEnvelope); err != nil {
				log.Printf("[NetworkManager] reply to %s failed: %v", pkt.Sender, err)
			}
		}(pkt)
	}
}

// ////
// Dispatch RPCs to any network e.g. udp or tcp
// Verifies the service and returns its handler from the registry
// //
func (m *NetworkManager) DispatchRPC(
	ctx context.Context,
	service api.AppLock,
	method string,
	payload []byte,
) ([]byte, error) {
	log.Printf("[NetworkManager]:%v Dispatching RPC", m.info.Username)
	// retrieve the handler from the registry and return it
	svcMap, ok := m.appHandlerRegistry[service]
	if !ok {
		return nil, fmt.Errorf("unknown service %q", service)
	}
	handler, ok := svcMap[method]
	if !ok {
		return nil, fmt.Errorf("unknown method %q for service %q", method, service)
	}
	return handler(ctx, payload)
}

// InvokeRPC sends a single RPC over UDP and waits for a reply.
func (m *NetworkManager) InvokeRPC(
	ctx context.Context,
	peerAddr string,
	service, method string,
	req, resp proto.Message,
) error {
	log.Printf("[NetworkManager]:%v Invoking RPC on %v", m.info.Username, peerAddr)

	// 1) Marshal the typed request
	payload, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal %s.%s: %w", service, method, err)
	}

	// 2) Wrap in your generic envelope
	envelope := &api.RPC{
		Service: service,
		Method:  method,
		Sender:  m.info,  // local node info
		Payload: payload, // actual typed RPC for the service
	}
	data, err := proto.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	// 3) Dial and send over UDP
	udpAddr, err := net.ResolveUDPAddr("udp", peerAddr)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", peerAddr, err)
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return fmt.Errorf("dial UDP %s: %w", peerAddr, err)
	}
	defer conn.Close()

	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("send to %s: %w", peerAddr, err)
	}

	// 4) Read response with a deadline
	buf := make([]byte, 64<<10)
	deadline := time.Now().Add(3 * time.Second)
	conn.SetReadDeadline(deadline)

	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		return fmt.Errorf("read from %s: %w", peerAddr, err)
	}

	// 5) Unmarshal the envelope
	var replyEnv api.RPC
	if err := proto.Unmarshal(buf[:n], &replyEnv); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	// 6) Finally unmarshal into the typed resp
	if err := proto.Unmarshal(replyEnv.Payload, resp); err != nil {
		return fmt.Errorf("unmarshal %s.%s response: %w", service, method, err)
	}

	// handle core services e.g. Kademlia routing
	if service == "services.KademliaService" {
		switch method {
		case "Ping":
			ack, ok := resp.(*services.ACK)
			if !ok {
				return fmt.Errorf("expected *services.ACK, got %T", resp)
			}
			m.lock.Lock()
			defer m.lock.Unlock()
			// record the Contact you just pinged
			// we already know its network address is peerAddr
			peer := ack.GetFrom()
			m.peers[peer.Username] = peer
			m.active[peer] = true

			log.Println(peerAddr, m.router.RoutingTable.RoutingTableString())

			// since we received an ack, add the peer to the routing table
			m.router.RoutingTable.Update(ctx, peer)
			log.Printf("[NetworkManager]:%v got %v.Ack@%v",
				m.info.Username, service, ack.From.Username)
			log.Println(m.router.RoutingTable.RoutingTableString())

		}
	}

	return nil
}
