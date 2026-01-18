package storage

import (
	"context"
	"sync"
	"time"

	"github.com/danmuck/dps_net/api"
	"github.com/danmuck/dps_net/api/services/router"
	key_store "github.com/danmuck/dps_net/storage/store"
)

type entry []byte

type Cache struct {
	data     map[api.NodeID]*entry
	accessed map[api.NodeID]time.Time
	size     int
	mu       sync.RWMutex
}

type LocalStorageServer struct {
	keyStore *key_store.KeyStore
	cache    *Cache

	accessKeys map[api.AppID]api.AppLock

	mu sync.RWMutex
}

func NewLocalStorage(storageDir string) (*LocalStorageServer, error) {
	ks, err := key_store.BootstrapKeyStore(storageDir)
	if err != nil {
		return nil, err
	}
	ls := &LocalStorageServer{
		keyStore: ks,
		cache: &Cache{
			data:     make(map[api.NodeID]*entry),
			accessed: make(map[api.NodeID]time.Time),
			size:     0,
		},
		accessKeys: make(map[api.AppID]api.AppLock),
	}
	return ls, nil

}

// type LocalStorage key_store.KeyStore

func (s *LocalStorageServer) PutFile(ctx context.Context, req *router.STORE, resp *router.ACK) error
func (s *LocalStorageServer) GetFile(ctx context.Context, req *router.FIND_VALUE, resp *router.VALUE) error

// etc.
// value must be a file to be chunked and stored on the dht
// func (ls *LocalStorageServer) PutFile(ctx context.Context, key api.NodeID, value []byte) error {
// 	return nil
// }

// func (ls *LocalStorageServer) GetFile(ctx context.Context, key api.NodeID) (value []byte, found bool, err error) {
// 	return nil, false, nil
// }

// key, value = ks.localChunks
func (ls *LocalStorageServer) SaveKey(ctx context.Context, key api.NodeID, value []byte) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	// ls.keyStore.WriteReferenceToDisk()
	return ls.keyStore.StoreKey(key, value)
}

func (ls *LocalStorageServer) FindKey(ctx context.Context, key api.NodeID) (value []byte, found bool, err error) {
	return nil, false, nil
}

// type NetworkStorage struct {
// 	keyStore key_store.KeyStore

// 	mu sync.RWMutex
// }

// func (ds *NetworkStorage) Save(ctx context.Context, key api.NodeID, value []byte) error {
// 	return nil
// }

// func (ds *NetworkStorage) Find(ctx context.Context, key api.NodeID) (value []byte, found bool, err error) {
// 	return nil, false, nil
// }
