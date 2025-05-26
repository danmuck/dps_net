package services

import (
	"context"

	"github.com/danmuck/dps_net/api"
	"github.com/danmuck/dps_net/network/routing"
)

// KademliaServiceImpl implements the KademliaService RPCs for peer discovery and DHT support.
type KademliaServiceImpl struct {
	rt *routing.RoutingTable
	// You can add storage or other dependencies here
}

// NewKademliaService creates a new service backed by the given routing table.
func NewKademliaService(rt *routing.RoutingTable) *KademliaServiceImpl {
	return &KademliaServiceImpl{rt: rt}
}

// Ping responds with an ACK echoing the value.
func (s *KademliaServiceImpl) Ping(ctx context.Context, req *PING) (*ACK, error) {
	// update routing: note the peer is alive
	s.rt.Update(ctx, req.From)
	return &ACK{From: req.From, Value: req.Value}, nil
}

// Store is a placeholder: you can hook in a DHT put here.
func (s *KademliaServiceImpl) Store(ctx context.Context, req *STORE) (*ACK, error) {
	// TODO: integrate with persistent storage
	s.rt.Update(ctx, req.From)
	return &ACK{From: req.From, Value: req.Value}, nil
}

// Delete is not supported yet.
func (s *KademliaServiceImpl) Delete(ctx context.Context, req *DELETE) (*ACK, error) {
	// TODO: integrate with persistent storage delete
	s.rt.Update(ctx, req.From)
	return &ACK{From: req.From, Value: nil}, nil
}

// FindNode returns up to k closest contacts to target_id.
func (s *KademliaServiceImpl) FindNode(ctx context.Context, req *FIND_NODE) (*NODES, error) {
	s.rt.Update(ctx, req.From)
	nodes, err := s.rt.FindClosestK(ctx, api.SliceToNodeID(req.TargetId))
	if err != nil {
		return nil, err
	}
	return &NODES{From: req.From, Nodes: nodes}, nil
}

// FindValue either returns the value or closest nodes.
func (s *KademliaServiceImpl) FindValue(ctx context.Context, req *FIND_VALUE) (*VALUE, error) {
	s.rt.Update(ctx, req.From)
	// TODO: check local storage for the key
	// if found: return &VALUE{From: req.From, Key: req.Key, Value: data}
	// else fallback to returning neighbors
	nodes, err := s.rt.FindClosestK(ctx, api.SliceToNodeID(req.Key))
	if err != nil {
		return nil, err
	}
	return &VALUE{From: req.From, Key: req.Key, Nodes: nodes}, nil
}

// Refresh re-publishes a key or bucket entry. Stub for optional behavior.
func (s *KademliaServiceImpl) Refresh(ctx context.Context, req *REFRESH) (*ACK, error) {
	s.rt.Update(ctx, req.From)
	// TODO: periodic refresh logic
	return &ACK{From: req.From, Value: nil}, nil
}
