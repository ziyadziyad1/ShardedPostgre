package types

import (
	"github.com/Mister-dev-oss/ShardedPostgre/validation"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Sends a query to a specific shard (TYPE = SELECT)
func (s *Shard) Query(q *Query) (pgx.Rows, error) {
	if err := validation.ValidateArgs(q.Sql, q.Args); err != nil {
		return nil, err
	}
	return s.Pool.Query(q.Ctx, q.Sql, q.Args...)
}

// Sends a query to a specific shard (TYPE != SELECT)
func (s *Shard) Exec(q *Query) (pgconn.CommandTag, error) {
	if err := validation.ValidateArgs(q.Sql, q.Args); err != nil {
		return pgconn.CommandTag{}, err
	}
	return s.Pool.Exec(q.Ctx, q.Sql, q.Args...)
}
