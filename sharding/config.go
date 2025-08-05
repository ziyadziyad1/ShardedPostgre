package sharding

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/Mister-dev-oss/ShardedPostgre/types"
)

// LoadConfig reads the JSON configuration file from the specified path and unmarshals it into a Config struct.
func LoadConfig(configPath string) (*types.Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg types.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ViewConfig loads the configuration from the given file and prints shard and vnode information to the console.
func ViewConfig(configPath string) error {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}

	fmt.Println("Number of Shards in Config: ", len(cfg.ConfigShards))
	fmt.Println("Vnodes for shard: ", cfg.Vnodes)
	for i, shard := range cfg.ConfigShards {
		fmt.Printf("Number: %d / Name: %s / Dns: %s\n", i+1, shard.Name, shard.Dns)
	}
	return nil
}

// SaveConfig serializes the given Config struct to JSON and writes it to the specified file path.
func SaveConfig(cfg *types.Config, configPath string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// ClearConfig resets the configuration file by writing an empty Config struct with no shards.
func ClearConfig(configPath string) error {
	empty := types.Config{
		ConfigShards: []types.ConfigShard{},
	}
	return SaveConfig(&empty, configPath)
}

// AddToConfig adds a new shard to the configuration file.
// WARNING: This function should NOT be used at runtime as it is not thread-safe.
func AddToConfig(s *types.ConfigShard, configPath string) error {
	config, err := LoadConfig(configPath)
	if err != nil {
		return err
	}

	for _, existing := range config.ConfigShards {
		if existing.Name == s.Name {
			return fmt.Errorf("Shard with name %s already exists", s.Name)
		}
	}

	config.ConfigShards = append(config.ConfigShards, *s)
	if err = SaveConfig(config, configPath); err != nil {
		return err
	}

	return nil
}

// RemoveFromConfig removes a shard from the configuration file by its name.
func RemoveFromConfig(s *types.ConfigShard, configPath string) error {
	config, err := LoadConfig(configPath)
	if err != nil {
		return err
	}

	newShards := make([]types.ConfigShard, 0, len(config.ConfigShards))
	for _, shard := range config.ConfigShards {
		if shard.Name != s.Name {
			newShards = append(newShards, shard)
		}
	}

	config.ConfigShards = newShards
	if err := SaveConfig(config, configPath); err != nil {
		return err
	}

	return nil
}

var configMu sync.Mutex

// SafeAddToConfig adds a shard to the config file in a thread-safe manner.
// This function can be safely used at runtime.
func SafeAddToConfig(s *types.ConfigShard, configPath string) error {
	configMu.Lock()
	defer configMu.Unlock()

	return AddToConfig(s, configPath)
}

// SafeRemoveFromConfig removes a shard from the config file in a thread-safe manner.
// This function can be safely used at runtime.
func SafeRemoveFromConfig(s *types.ConfigShard, configPath string) error {
	configMu.Lock()
	defer configMu.Unlock()

	return RemoveFromConfig(s, configPath)
}

// ModifyConfigVnodes updates the number of virtual nodes (vnodes) in the configuration file.
// WARNING: Do NOT use this function at runtime as it may cause incorrect shard balancing.
func ModifyConfigVnodes(Vnodes int, configPath string) error {
	config, err := LoadConfig(configPath)
	if err != nil {
		return err
	}
	config.Vnodes = Vnodes
	if err = SaveConfig(config, configPath); err != nil {
		return err
	}

	return nil
}
