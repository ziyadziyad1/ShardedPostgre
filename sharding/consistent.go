package sharding

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/Mister-dev-oss/ShardedPostgre/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spaolacci/murmur3"
)

// HashRing represents a consistent hashing ring with virtual nodes (vnodes) for sharding.
// It maintains a sorted list of hashes and a mapping from hash to shard name.
// It supports concurrent access with a read-write mutex.
type HashRing struct {
	mu           sync.RWMutex
	SortedHashes []uint32
	HashToShard  map[uint32]string
	Shards       []types.Shard
	Vnodes       int
}

// emptyHashRing creates an empty HashRing with the specified number of virtual nodes.
func emptyHashRing(vnodes int) *HashRing {
	return &HashRing{
		SortedHashes: []uint32{},
		HashToShard:  make(map[uint32]string),
		Vnodes:       vnodes,
	}
}

// InitHashRing initializes a HashRing from a configuration file path.
// It loads the config, creates connection pools for each shard, and adds them to the ring.
func InitHashRing(configPath string) (*HashRing, error) {
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	hashRing := emptyHashRing(config.Vnodes)

	for _, configShard := range config.ConfigShards {
		pool, err := pgxpool.New(context.Background(), configShard.Dns)
		if err != nil {
			return nil, fmt.Errorf("error creating pool for shard %s: %v", configShard.Name, err)
		}

		shard := types.Shard{Name: configShard.Name, Pool: pool}
		hashRing.AddShard(&shard)
	}

	return hashRing, nil
}

// AddShard adds a shard to the hash ring with its virtual nodes.
// It locks the ring for writing, computes vnode hashes, updates mappings, and sorts the hashes.
func (h *HashRing) AddShard(shard *types.Shard) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i := 0; i < h.Vnodes; i++ {
		VnodeKey := fmt.Sprintf("%s#%d", shard.Name, i)
		hash := murmur3.Sum32([]byte(VnodeKey))
		h.SortedHashes = append(h.SortedHashes, hash)
		h.HashToShard[hash] = shard.Name
	}
	h.Shards = append(h.Shards, *shard)
	slices.Sort(h.SortedHashes)
}

// RemoveShard removes a shard and its virtual nodes from the hash ring.
// It locks the ring for writing, filters out vnode hashes and updates shard list.
func (h *HashRing) RemoveShard(shard *types.Shard) {
	h.mu.Lock()
	defer h.mu.Unlock()

	filteredHashes := h.SortedHashes[:0]
	for _, hash := range h.SortedHashes {
		if h.HashToShard[hash] != shard.Name {
			filteredHashes = append(filteredHashes, hash)
		} else {
			delete(h.HashToShard, hash)
		}
	}
	h.SortedHashes = filteredHashes

	newShards := make([]types.Shard, 0, len(h.Shards))
	for _, s := range h.Shards {
		if s.Name != shard.Name {
			newShards = append(newShards, s)
		}
	}
	h.Shards = newShards
}

// GetShard returns the shard name responsible for the given key.
// It uses consistent hashing to find the closest vnode hash clockwise on the ring.
func (h *HashRing) GetShard(key string) (string, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.SortedHashes) == 0 {
		return "", fmt.Errorf("ring is empty")
	}

	hash := murmur3.Sum32([]byte(key))
	idx := sort.Search(len(h.SortedHashes), func(i int) bool {
		return h.SortedHashes[i] >= hash
	})

	if idx == len(h.SortedHashes) {
		idx = 0
	}
	vnodeHash := h.SortedHashes[idx]
	return h.HashToShard[vnodeHash], nil
}

// Clone creates a deep copy of the HashRing.
// It copies the sorted hashes, hash-to-shard map, and shard slice.
func (h *HashRing) Clone() *HashRing {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Copy SortedHashes
	newHashes := make([]uint32, len(h.SortedHashes))
	copy(newHashes, h.SortedHashes)

	// Copy HashToShard map
	newMap := make(map[uint32]string, len(h.HashToShard))
	for k, v := range h.HashToShard {
		newMap[k] = v
	}

	// Copy Shards slice (shallow copy of structs)
	newShards := make([]types.Shard, len(h.Shards))
	copy(newShards, h.Shards)

	return &HashRing{
		SortedHashes: newHashes,
		HashToShard:  newMap,
		Shards:       newShards,
		Vnodes:       h.Vnodes,
	}
}

// RingManager manages a HashRing instance with concurrency control and configuration path.
// It provides methods to add and remove shards safely, updating both the ring and config file.
// It also maintains an atomic pointer to the current active ring.
type RingManager struct {
	mu         sync.Mutex
	Ring       *HashRing
	ConfigPath string
	Rring      atomic.Pointer[HashRing]
}

// InitRingManager initializes a RingManager with a given config path.
// It loads the initial hash ring and stores it atomically.
func InitRingManager(configpath string) (*RingManager, error) {
	hashRing, err := InitHashRing(configpath)
	if err != nil {
		return nil, err
	}

	rm := &RingManager{
		Ring:       hashRing,
		ConfigPath: configpath,
	}
	rm.Rring.Store(hashRing)

	return rm, nil
}

// AddShard adds a new shard to the ring manager.
// It creates a connection pool, clones the current ring, adds the shard, updates the active ring,
// and safely updates the configuration file.
func (rm *RingManager) AddShard(name, dns string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Create connection pool for the new shard
	pool, err := pgxpool.New(context.Background(), dns)
	if err != nil {
		return err
	}

	newShard := types.Shard{Name: name, Pool: pool}

	// Clone the current ring (deep copy)
	newRing := rm.Ring.Clone()
	newRing.AddShard(&newShard)

	// Update the active ring
	rm.Ring = newRing
	rm.Rring.Store(newRing)

	// Also add the shard to the JSON configuration file safely
	return SafeAddToConfig(&types.ConfigShard{Name: name, Dns: dns}, rm.ConfigPath)
}

// RemoveShard removes a shard by name from the ring manager.
// It locks the manager, finds the shard, clones the ring, removes the shard,
// updates the active ring, and safely updates the configuration file.
func (rm *RingManager) RemoveShard(name string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	condition := false
	var newshard types.Shard

	// Find the shard in the current ring
	for _, shard := range rm.Ring.Shards {
		if shard.Name == name {
			condition = true
			newshard = shard
			break
		}
	}

	if condition {

		config, err := LoadConfig(rm.ConfigPath)
		if err != nil {
			return err
		}

		var configShard types.ConfigShard

		// Find the shard in the config
		found := false
		for _, cShard := range config.ConfigShards {
			if cShard.Name == name {
				configShard = cShard
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("Name not found in config")
		}

		// Clone the current ring (deep copy)
		newRing := rm.Ring.Clone()
		newRing.RemoveShard(&newshard)

		// Update the active ring
		rm.Ring = newRing
		rm.Rring.Store(newRing)

		// Also remove the shard from the JSON configuration file safely
		return SafeRemoveFromConfig(&configShard, rm.ConfigPath)
	}

	return fmt.Errorf("Name isn't in shards list")
}

// Query executes a query that returns multiple rows on the appropriate shard based on the key.
func (rm *RingManager) Query(q types.Query) (pgx.Rows, error) {
	ring := rm.Rring.Load()
	if ring == nil {
		return nil, fmt.Errorf("ring is not initialized")
	}

	shardName, err := ring.GetShard(q.Key)
	if err != nil {
		return nil, err
	}

	for _, shard := range ring.Shards {
		if shard.Name == shardName {
			return shard.Query(&q)
		}
	}

	return nil, fmt.Errorf("Error in sharding config/state, shardname %s not found", shardName)
}

// Exec executes a query that does not return rows on the appropriate shard based on the key.
func (rm *RingManager) Exec(q types.Query) (pgconn.CommandTag, error) {
	ring := rm.Rring.Load()
	if ring == nil {
		return pgconn.CommandTag{}, fmt.Errorf("ring is not initialized")
	}

	shardName, err := ring.GetShard(q.Key)
	if err != nil {
		return pgconn.CommandTag{}, err
	}

	for _, shard := range ring.Shards {
		if shard.Name == shardName {
			return shard.Exec(&q)
		}
	}

	return pgconn.CommandTag{}, fmt.Errorf("Error in sharding config/state, shardname %s not found", shardName)
}

// QueryRow executes a query that returns a single row on the appropriate shard based on the key.
func (rm *RingManager) QueryRow(q types.Query) (pgx.Row, error) {
	ring := rm.Rring.Load()
	if ring == nil {
		return nil, fmt.Errorf("ring is not initialized")
	}

	shardName, err := ring.GetShard(q.Key)
	if err != nil {
		return nil, err
	}

	for _, shard := range ring.Shards {
		if shard.Name == shardName {
			return shard.QueryRow(&q)
		}
	}

	return nil, fmt.Errorf("Error in sharding config/state, shardname %s not found", shardName)
}
