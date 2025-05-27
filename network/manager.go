package network

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/danmuck/dps_net/api"
	"github.com/danmuck/dps_net/config"
	"github.com/danmuck/dps_net/network/routing"
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
	nodeID    api.NodeID            // local nodeID
	localAddr string                // local network address <ip:port>
	router    *routing.RoutingTable // p2p network routing table
	info      *api.Contact

	receiver chan transport.Packet
	udpServ  transport.Server // UDP packet server
	tcpServ  transport.Server // TCP packet server

	peers              map[string]*api.Contact                // username -> contact
	active             map[*api.Contact]bool                  // contact -> isActive
	trusted            map[string]string                      // TODO: username -> trust key
	appLocks           map[string]api.AppLock                 // appID -> appLock
	appHandlerRegistry map[api.AppLock]map[string]RPC_Handler // name -> rpc name -> handler
	cfg                *config.Config

	lock sync.RWMutex
}

// ////
// Initialize a new NetworkManager for a Node
// //
func NewNetworkManager(local *api.Contact, cfg config.Config) (*NetworkManager, error) {

	nm := &NetworkManager{
		nodeID:    local.ID(),
		localAddr: local.GetAddress(),
		router:    routing.NewRoutingTable(local, cfg.K, cfg.Alpha),
		info:      local,

		receiver: nil,
		udpServ:  nil,
		tcpServ:  nil,

		peers:              make(map[string]*api.Contact),
		active:             make(map[*api.Contact]bool),
		appLocks:           make(map[string]api.AppLock),
		appHandlerRegistry: make(map[api.AppLock]map[string]RPC_Handler),
		cfg:                &cfg,
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

	kad := routing.KademliaService_ServiceDesc
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

	// nm.tcpServ = grpc.NewServer()
	// services.RegisterKademliaServiceServer(m.grpcServer, rt)

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
			log.Printf("[NetworkManager] @%v packet received for %v", m.info.Username, pkt.Sender)

			appLock := m.appLocks[pkt.RPC.Service]
			log.Printf("[NetworkManager] @%v Dispatching RPC %s.%s",
				m.info.Username, pkt.RPC.Service, pkt.RPC.Method)
			// delegate lookup & call to DispatchRPC
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
			m.router.Update(pkt.Ctx, pkt.Sender)

			if err := pkt.Reply(replyEnvelope); err != nil {
				log.Printf("[NetworkManager] reply to %s failed: %v", pkt.Sender, err)
			}
		}(pkt)
	}
}

// Lookup performs the Kademlia iterative FindNode for targetID,
// using only the peers already in your routing table.  It returns
// up to k closest Contacts to targetID.
func (m *NetworkManager) Lookup(ctx context.Context, target api.NodeID) ([]*api.Contact, error) {
	svcName := routing.KademliaService_ServiceDesc.ServiceName

	// 1) seed shortlist from local routing table
	shortlist, err := m.router.ClosestK(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("initial FindClosestK: %w", err)
	}

	k := m.cfg.K         // bucket size
	alpha := m.cfg.Alpha // parallelism

	queried := make(map[string]bool)
	var prevFurthest api.NodeID

	for {
		if len(shortlist) == 0 {
			return nil, nil
		}
		// sort & truncate to k
		sort.Slice(shortlist, func(i, j int) bool {
			return api.CompareXorDistance(
				api.NodeID(shortlist[i].GetId()),
				api.NodeID(shortlist[j].GetId()),
				target,
			)
		})
		if len(shortlist) > k {
			shortlist = shortlist[:k]
		}

		// have we attained a closer node than furthest?
		furthest := api.XorDistance(
			api.NodeID(shortlist[len(shortlist)-1].GetId()),
			target,
		)
		if prevFurthest != (api.NodeID{}) && !api.LessDistance(furthest, prevFurthest) {
			break
		}
		prevFurthest = furthest

		// pick up to αlpha unqueried peers
		toQuery := make([]*api.Contact, 0, alpha)
		for _, c := range shortlist {
			addr := c.GetUDPAddress()
			if !queried[addr] && len(toQuery) < alpha {
				queried[addr] = true
				toQuery = append(toQuery, c)
			}
		}
		if len(toQuery) == 0 {
			break
		}

		// parallel FindNode on each
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, c := range toQuery {
			wg.Add(1)
			go func(c *api.Contact) {
				defer wg.Done()
				var resp routing.NODES
				err := m.InvokeRPC(
					ctx,
					c.GetUDPAddress(),
					svcName, "FindNode",
					&routing.FIND_NODE{
						From:     m.info,
						TargetId: target[:],
					},
					&resp,
				)
				if err != nil {
					return
				}
				mu.Lock()
				for i := range resp.Nodes {
					shortlist = append(shortlist, resp.Nodes[i])
				}
				mu.Unlock()
			}(c)
		}

		// wait with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}

	// final sort & trim
	sort.Slice(shortlist, func(i, j int) bool {
		return api.CompareXorDistance(
			api.NodeID(shortlist[i].GetId()),
			api.NodeID(shortlist[j].GetId()),
			target,
		)
	})
	if len(shortlist) > k {
		shortlist = shortlist[:k]
	}
	return shortlist, nil
}
