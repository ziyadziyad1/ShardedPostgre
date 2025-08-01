package sharding

import (
	"fmt"
	"slices"
	"sort"
	"sync"

	"github.com/Mister-dev-oss/ShardedPostgre/types"
	"github.com/spaolacci/murmur3"
)

// HashRing struct
type HashRing struct {
	mu           sync.RWMutex
	SortedHashes []uint32
	HashToShard  map[uint32]string
	Shards       []types.Shard
	Vnodes       int
}

func emptyHashRing(vnodes int) *HashRing {
	return &HashRing{
		SortedHashes: []uint32{},
		HashToShard:  make(map[uint32]string),
		Vnodes:       vnodes,
	}
}

func InitHashRing() (*HashRing, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	hashRing := emptyHashRing(config.Vnodes)
	for _, Shards := range config.ConfigShards {
		hashRing.AddNode(Shards.Name)
	}
	return hashRing, nil
}

func (h *HashRing) AddNode(node string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i := 0; i < h.Vnodes; i++ {
		VnodeKey := fmt.Sprintf("%s#%d", node, i)
		hash := murmur3.Sum32([]byte(VnodeKey))
		h.SortedHashes = append(h.SortedHashes, hash)
		h.HashToShard[hash] = node
	}
	slices.Sort(h.SortedHashes)
}

func (h *HashRing) RemoveNode(node string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	filteredHashes := h.SortedHashes[:0]
	for _, hash := range h.SortedHashes {
		if h.HashToShard[hash] != node {
			filteredHashes = append(filteredHashes, hash)
		} else {
			delete(h.HashToShard, hash)
		}
	}
	h.SortedHashes = filteredHashes
}

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
