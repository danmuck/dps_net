package network

import (
	"context"
	"fmt"
	"log"

	"github.com/danmuck/dps_net/api"
)

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
	log.Printf("[NetworkManager] @%v Dispatching RPC %v", m.info.Username, method)

	// TODO: Middleware

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
