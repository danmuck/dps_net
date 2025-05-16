package node

import (
	"sync"

	"github.com/danmuck/dps_net/api"
)

type LocalStorage struct {
	accessKeys map[api.AppID]api.AppLock
	data       map[api.AppLock]DataStorage

	lock sync.RWMutex
}

type DataStorage struct {
	data map[api.NodeID][]byte

	lock sync.RWMutex
}
