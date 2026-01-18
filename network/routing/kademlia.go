package routing

import (
	context "context"

	"github.com/danmuck/dps_net/api"
	"github.com/danmuck/dps_net/api/services/router"
)

// NewKademliaService creates a new service backed by the given routing table.
// func NewKademliaService(rt *RoutingTable) *RoutingTable {
// 	return &RoutingTable{}
// }

// Ping responds with an router.ACK echoing the value.
func (s *RoutingTable) Ping(ctx context.Context, req *router.PING) (*router.ACK, error) {
	// update routing: note the peer is alive
	s.Update(ctx, req.From)
	return &router.ACK{From: s.GetLocal(), Value: req.Value}, nil
}

// Store is a placeholder: you can hook in a DHT put here.
func (s *RoutingTable) Store(ctx context.Context, req *router.STORE) (*router.ACK, error) {
	// TODO: integrate with persistent storage
	s.Update(ctx, req.From)
	return &router.ACK{From: s.GetLocal(), Value: req.Value}, nil
}

// Delete is not supported yet.
func (s *RoutingTable) Delete(ctx context.Context, req *router.DELETE) (*router.ACK, error) {
	// TODO: integrate with persistent storage delete
	s.Update(ctx, req.From)
	return &router.ACK{From: s.GetLocal(), Value: nil}, nil
}

// FindNode returns up to k closest contacts to target_id.
func (s *RoutingTable) FindNode(ctx context.Context, req *router.FIND_NODE) (*router.NODES, error) {
	s.Update(ctx, req.From)
	nodes, err := s.ClosestK(ctx, api.SliceToNodeID(req.TargetId))
	if err != nil {
		return nil, err
	}
	return &router.NODES{
		From:  s.GetLocal(),
		Nodes: nodes,
	}, nil
}

// FindValue either returns the value or closest nodes.
func (s *RoutingTable) FindValue(ctx context.Context, req *router.FIND_VALUE) (*router.VALUE, error) {
	s.Update(ctx, req.From)
	// TODO: check local storage for the key
	// if found: return &VALUE{From: req.From, Key: req.Key, Value: data}
	// else fallback to returning neighbors
	nodes, err := s.ClosestK(ctx, api.SliceToNodeID(req.Key))
	if err != nil {
		return nil, err
	}
	return &router.VALUE{
		From:  s.GetLocal(),
		Key:   req.Key,
		Nodes: nodes,
	}, nil
}

// Refresh re-publishes a key or bucket entry. Stub for optional behavior.
func (s *RoutingTable) Refresh(ctx context.Context, req *router.REFRESH) (*router.ACK, error) {
	s.Update(ctx, req.From)
	// TODO: periodic refresh logic
	return &router.ACK{From: s.GetLocal(), Value: nil}, nil
}
