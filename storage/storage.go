package node

import (
	"context"
	"sync"

	"github.com/danmuck/dps_net/api"
)

type LocalStorage struct {
	accessKeys map[api.AppID]api.AppLock
	data       map[api.AppLock]DataStorage

	lock sync.RWMutex
}

func (ls *LocalStorage) Save(ctx context.Context, key api.NodeID, value []byte) error {
	return nil
}

func (ls *LocalStorage) Find(ctx context.Context, key api.NodeID) (value []byte, found bool, err error) {
	return nil, false, nil
}

type DataStorage struct {
	data map[api.NodeID][]byte

	lock sync.RWMutex
}

func (ds *DataStorage) Save(ctx context.Context, key api.NodeID, value []byte) error {
	return nil
}

func (ds *DataStorage) Find(ctx context.Context, key api.NodeID) (value []byte, found bool, err error) {
	return nil, false, nil
}
