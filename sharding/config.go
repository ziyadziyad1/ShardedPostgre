package sharding

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Mister-dev-oss/ShardedPostgre/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config.Shards Initializing
func InitializeShards() ([]types.Shard, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	shards := []types.Shard{}

	for _, configshard := range config.ConfigShards {
		pool, err := pgxpool.New(context.Background(), configshard.Dns)
		if err != nil {
			return nil, err
		}

		shard := types.Shard{
			Name: configshard.Name,
			Pool: pool,
		}

		shards = append(shards, shard)

	}

	return shards, nil

}

// Config.json reading, get info about actual sharding config
func LoadConfig() (*types.Config, error) {
	var configPath = "./sharding/config.json"
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

// Config.json viewing and infos (console view)
func ViewConfig() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	fmt.Println("Number of Shards in Config: ", len(cfg.ConfigShards))
	fmt.Println("Vnodes for shard: ", cfg.Vnodes)
	for i, shard := range cfg.ConfigShards {
		fmt.Printf("Number: %d / Name: %s / Dns: %s", i+1, shard.Name, shard.Dns)
	}
	return nil
}

// Config.json writing, add or remove existing sharding config data
func SaveConfig(cfg *types.Config) error {
	var configPath = "./sharding/config.json"
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// Config.json fully clear your sharding config
func ClearConfig() error {
	empty := types.Config{
		ConfigShards: []types.ConfigShard{},
	}
	return SaveConfig(&empty)
}

func AddToConfig(s *types.ConfigShard) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	config.ConfigShards = append(config.ConfigShards, *s)
	if err = SaveConfig(config); err != nil {
		return err
	}

	return nil
}

func ModifyConfigVnodes(Vnodes int) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}
	config.Vnodes = Vnodes
	if err = SaveConfig(config); err != nil {
		return err
	}

	return nil
}
