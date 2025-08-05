package types

import (
	"context"
	"time"

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

func (s *Shard) QueryRow(q *Query) (pgx.Row, error) {
	if err := validation.ValidateArgs(q.Sql, q.Args); err != nil {
		return nil, err
	}
	return s.Pool.QueryRow(q.Ctx, q.Sql, q.Args...), nil
}

func (s *Shard) HealthCheck() bool {
	if err := s.Pool.Ping(context.Background()); err != nil {
		return false
	}
	return true
}

func (s *Shard) RetryHealthCheck(retries int, delay time.Duration) bool {
	for i := 0; i < retries; i++ {
		if s.Pool.Ping(context.Background()) == nil {
			return true
		}
		time.Sleep(delay)
	}
	return false
}
