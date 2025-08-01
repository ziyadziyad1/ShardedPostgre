package types

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Shard struct {
	Name string
	Pool *pgxpool.Pool
}

type ConfigShard struct {
	Name string `json:"name"`
	Dns  string `json:"Dns"`
}

type Config struct {
	ConfigShards []ConfigShard
	Vnodes       int
}

type Query struct {
	Ctx  context.Context
	Sql  string
	Args []any
}
