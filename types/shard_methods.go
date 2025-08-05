package types

import (
	"context"
	"time"

	"github.com/Mister-dev-oss/ShardedPostgre/validation"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Query sends a SELECT query to a specific shard.
// It validates the query arguments before executing.
func (s *Shard) Query(q *Query) (pgx.Rows, error) {
	if err := validation.ValidateArgs(q.Sql, q.Args); err != nil {
		return nil, err
	}
	return s.Pool.Query(q.Ctx, q.Sql, q.Args...)
}

// Exec sends a non-SELECT query (e.g., INSERT, UPDATE, DELETE) to a specific shard.
// It validates the query arguments before executing.
func (s *Shard) Exec(q *Query) (pgconn.CommandTag, error) {
	if err := validation.ValidateArgs(q.Sql, q.Args); err != nil {
		return pgconn.CommandTag{}, err
	}
	return s.Pool.Exec(q.Ctx, q.Sql, q.Args...)
}

// QueryRow sends a query expected to return a single row to a specific shard.
// It validates the query arguments before executing.
func (s *Shard) QueryRow(q *Query) (pgx.Row, error) {
	if err := validation.ValidateArgs(q.Sql, q.Args); err != nil {
		return nil, err
	}
	return s.Pool.QueryRow(q.Ctx, q.Sql, q.Args...), nil
}

// HealthCheck checks if the shard's connection pool is reachable by pinging it.
// Returns true if the ping succeeds, false otherwise.
func (s *Shard) HealthCheck() bool {
	if err := s.Pool.Ping(context.Background()); err != nil {
		return false
	}
	return true
}

// RetryHealthCheck attempts to ping the shard multiple times with a delay between attempts.
// Returns true if any ping succeeds within the retries, false otherwise.
func (s *Shard) RetryHealthCheck(retries int, delay time.Duration) bool {
	for i := 0; i < retries; i++ {
		if s.Pool.Ping(context.Background()) == nil {
			return true
		}
		time.Sleep(delay)
	}
	return false
}
