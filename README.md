https://github.com/ziyadziyad1/ShardedPostgre/releases

# ShardedPostgre: Consistent Hashing & Virtual Nodes for PostgreSQL

[![Release](https://img.shields.io/badge/Release-Download-blue?style=for-the-badge&logo=github&labelColor=282c34)](https://github.com/ziyadziyad1/ShardedPostgre/releases)

[![Go](https://img.shields.io/badge/Language-Go-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/DB-PostgreSQL-336791?style=flat-square&logo=postgresql)](https://www.postgresql.org)
[![Topics](https://img.shields.io/badge/Topics-consistent--hashing%20%7C%20sharding%20%7C%20scaling-6f42c1?style=flat-square)](#)
[![License](https://img.shields.io/badge/License-MIT-007ec6?style=flat-square)](LICENSE)

![Postgres Elephant](https://upload.wikimedia.org/wikipedia/commons/2/29/Postgresql_elephant.svg)

ShardedPostgre is a Go library that implements consistent hashing with virtual nodes to split PostgreSQL data across multiple instances. The library targets services that need horizontal scaling for write and read workloads while keeping routing logic simple and stable.

Download and execute the release file from: https://github.com/ziyadziyad1/ShardedPostgre/releases

Table of Contents

- What this library does
- Key concepts
- Architecture overview
- Design goals
- Getting started
  - Install
  - Quick start example
  - Release download and execution
- Core API
  - Ring and node types
  - Connection helpers
  - Query routing
  - Migrations and rebalancing
- Example setups
  - Local Docker Compose with three shards
  - Kubernetes StatefulSet pattern
- Schema and migration strategy
- Shard-aware operations
  - Single-shard writes
  - Multi-shard queries
  - Cross-shard joins and patterns
- Consistent hashing internals
  - Hash functions
  - Virtual nodes and load balance math
  - Handling hotspots and skew
- Failure handling and recovery
  - Node removal and addition
  - Failover patterns
  - Backoff and retry rules
- Performance and benchmarks
  - Latency model
  - Throughput model
  - Memory and CPU notes
- Testing
  - Unit tests
  - Integration tests
  - Chaos testing
- Migration and rebalancing guide
- Security notes
- Observability and metrics
- Best practices
- FAQ
- Contributing
- License

What this library does

- Route client requests to the correct PostgreSQL instance based on a key.
- Use consistent hashing to map keys to shards.
- Use virtual nodes to reduce variance in load distribution.
- Provide connection pooling helpers and retry logic.
- Provide migration helpers to move ranges between shards.
- Provide testing utilities and a CLI for operational tasks.

Key concepts

- Shard: A single PostgreSQL instance that stores a subset of data.
- Ring: A virtual hash ring used to assign keys to shards.
- Virtual node (vnode): A pointer on the ring that maps to a physical shard. Each physical shard owns multiple vnodes.
- Key: A value (such as user ID) that determines where data lives.
- Token: Hash value derived from key or vnode identifier.
- Routing: The act of finding the shard for a key.
- Rebalance: Moving vnodes or data to change distribution.

Architecture overview

ShardedPostgre uses a client-side ring. Each service node loads ring metadata. The ring maps many virtual tokens to physical shards. The client computes a hash for a key, finds the next vnode in clockwise order, and routes to that vnode's physical shard. The library keeps a connection pool per shard. The library exposes an API that returns a *sql.DB or a wrapper for use in application code.

Design goals

- Stable routing. When a shard changes, only a small subset of keys move.
- Even load. Virtual nodes aim to balance keys across shards.
- Simple API. Keep the surface small for common routing tasks.
- Operability. Provide CLI and helpers for migration and metrics.
- Testability. Provide tools to simulate failures and rebalancing.

Getting started

Install

Use go modules. Import the package path for your project. Example:

go get github.com/ziyadziyad1/ShardedPostgre

Quick start example

This example shows a minimal use. It creates a ring with three shards and routes a key.

package main

import (
  "context"
  "database/sql"
  "fmt"
  "time"

  "github.com/ziyadziyad1/ShardedPostgre"
  _ "github.com/lib/pq"
)

func main() {
  cfg := shardedpostgres.Config{
    VNodes:     256,
    HashFunc:   shardedpostgres.HashMurmur3,
    ConnectTTL: 10 * time.Second,
  }

  shards := []shardedpostgres.NodeConfig{
    {ID: "shard-a", DSN: "postgres://user:pass@127.0.0.1:5432/db_a?sslmode=disable"},
    {ID: "shard-b", DSN: "postgres://user:pass@127.0.0.1:5433/db_b?sslmode=disable"},
    {ID: "shard-c", DSN: "postgres://user:pass@127.0.0.1:5434/db_c?sslmode=disable"},
  }

  ring, err := shardedpostgres.NewRing(cfg, shards...)
  if err != nil {
    panic(err)
  }

  key := "user:12345"
  node := ring.GetNode(key)
  db := ring.DB(node.ID) // returns *sql.DB or wrapper

  var name string
  err = db.QueryRowContext(context.Background(), "SELECT name FROM users WHERE id = $1", 12345).Scan(&name)
  if err != nil {
    fmt.Println("query error:", err)
    return
  }
  fmt.Println("user name:", name)
}

Release download and execution

Download the release file and execute it. Use the releases page at:
https://github.com/ziyadziyad1/ShardedPostgre/releases

Find the release tarball or binary that matches your OS. Download the file and run the binary, or extract the archive to access the CLI tools. The release artifacts include a CLI called spgctl that can print ring metadata, run migrations, and perform health checks. After downloading, make the binary executable and run it:

chmod +x spgctl-linux-amd64
./spgctl-linux-amd64 version
./spgctl-linux-amd64 ring inspect --config ring.yaml

Core API

Types

- Config: Library-level options.
  - VNodes: number of virtual nodes per physical node.
  - HashFunc: which hash function to use.
  - ConnectTTL: TTL for per-shard connections.
  - MaxOpenConns, MaxIdleConns: DB pool config.

- NodeConfig: info needed to connect to a shard.
  - ID: unique shard identifier.
  - DSN: PostgreSQL DSN.
  - Weight: optional integer to reflect capacity differences.
  - Region: optional locality meta.

- Ring: primary object that holds tokens and maps to nodes.
  - AddNode(NodeConfig)
  - RemoveNode(nodeID)
  - GetNode(key) -> Node
  - DB(nodeID) -> *sql.DB
  - Close()

Hash functions

ShardedPostgre ships with a set of tested hash functions:

- HashMurmur3 – good distribution for keyspace.
- HashFNV1a – simple and compact.
- HashXXHash – fast and solid.

You can pass a custom function of type func([]byte) uint64.

Ring lifecycle

- NewRing(cfg, nodes...) builds ring and opens DB pools.
- ring.AddNode adds nodes and places vnodes on the ring.
- ring.RemoveNode removes node and marks vnodes for migration.
- ring.Rebalance triggers a controlled redistribution.
- ring.Snapshot returns the ring map for external storage.

Connection helpers

- ring.DB(nodeID) returns a *sql.DB that uses connection pool settings.
- ring.PoolStats(nodeID) returns connection stats.
- ring.ExecOnNode(nodeID, query, args...) executes a write on a specific node.

Query routing

- ring.GetNode(key) returns the NodeConfig for a key.
- ring.RouteQuery(ctx, key, func(db *sql.DB) error) opens a connection and runs the closure.
- ring.MultiQuery(ctx, keys, func(nodeID string, db *sql.DB) error) runs per-shard function concurrently.

Migrations and rebalancing

- ring.MarkForMigration(nodeID, tokenRanges) marks tokens that must move.
- ring.StartMigration(ctx, plan) runs migration steps and verifies checksums.
- ring.RollbackMigration(planID) rolls back a failed migration.

Example: adding a node

cfg := shardedpostgres.Config{VNodes: 512}
ring, _ := shardedpostgres.NewRing(cfg, existingNodes...)
ring.AddNode(shardedpostgres.NodeConfig{ID: "shard-d", DSN: "...", Weight: 1})
ring.Rebalance(context.Background(), shardedpostgres.RebalanceOptions{ConcurrentMoves: 2})

Example: routing a write

err := ring.RouteQuery(ctx, "order:9876", func(db *sql.DB) error {
  _, err := db.ExecContext(ctx, "INSERT INTO orders(id, amount) VALUES ($1, $2)", 9876, 42.5)
  return err
})

Example: multi-shard read

keys := []string{"user:1", "user:2", "user:3"}
err := ring.MultiQuery(ctx, keys, func(nodeID string, db *sql.DB) error {
  rows, err := db.QueryContext(ctx, "SELECT id, name FROM users WHERE id = ANY($1)", pq.Array(idsForNode(nodeID)))
  if err != nil {
    return err
  }
  defer rows.Close()
  // process rows
  return nil
})

Example setups

Local Docker Compose with three shards

This compose brings up three Postgres containers on ports 5432, 5433, 5434. It creates databases db_a, db_b, db_c.

version: "3.7"
services:
  postgres-a:
    image: postgres:14
    environment:
      POSTGRES_USER: user
      POSTGRES_PASSWORD: pass
      POSTGRES_DB: db_a
    ports:
      - "5432:5432"
  postgres-b:
    image: postgres:14
    environment:
      POSTGRES_USER: user
      POSTGRES_PASSWORD: pass
      POSTGRES_DB: db_b
    ports:
      - "5433:5432"
  postgres-c:
    image: postgres:14
    environment:
      POSTGRES_USER: user
      POSTGRES_PASSWORD: pass
      POSTGRES_DB: db_c
    ports:
      - "5434:5432"

After starting, run migrations on each DB to create the schema. Then configure the ring with DSNs pointing to each port.

Kubernetes StatefulSet pattern

- Use a StatefulSet for each shard with a stable DNS name.
- Use PVCs for persistence.
- Use liveness and readiness probes.
- Create a ConfigMap that holds ring metadata (token map). Keep ring metadata in an external store such as etcd, Consul, or a GitOps-managed file.

Schema and migration strategy

Sharding does not change schema design. It changes where rows live. Use a shard key that maps one row to one shard. Common keys: user_id, tenant_id, account_id.

- Single-table per shard: apply schema on each shard. Keep DDL synchronized.
- Global tables: keep small global metadata in one place or replicate it across shards.
- Secondary indexes: apply on each shard.
- Unique constraints across shards: impossible without coordination. Use a coordinator for global uniqueness or use a composite key with shard prefix.

Migration steps for moving data between shards

1. Plan token moves at vnode level.
2. Create migration job with source and target nodes and token range.
3. Run initial copy with consistent snapshot.
4. Apply a change capture for live writes (logical decoding or WAL streaming).
5. Switch routing rules for the moved token range.
6. Drain leftover changes and finalize.

Shard-aware operations

Single-shard writes

Design operations so each write targets a single shard. This keeps transactions local and fast.

Transactions that span shards require a distributed transaction pattern. Use careful compensation or a two-phase commit if needed. Avoid multi-shard transactions unless needed.

Multi-shard queries

Aggregate queries that must touch multiple shards require either:

- Query fan-out: send subqueries to each shard, aggregate in the app.
- Data lake: copy sharded data into a central store for analytics.

Cross-shard joins and patterns

Avoid joins across shards. If you need cross-shard joins, you have these options:

- Ship small numerator table to each shard and perform local join.
- Use application-side join after fan-out.
- Maintain a denormalized global index.

Consistent hashing internals

How the ring works

- For each physical shard, create N vnodes. Each vnode gets a token H(shardID || vnodeIndex).
- Sort tokens in ascending order.
- To map a key, compute T = H(key) and find the first token >= T. If none, wrap to the first token.
- The token maps to its physical shard.

Hash functions

Pick a hash with good distribution. Murmur3 and xxHash give strong spread. For security-sensitive cases, pick a cryptographic hash, but expect slower performance.

Virtual nodes and load balance math

Let S be the number of physical shards and V be vnodes per shard. Total tokens = S * V. If keys are uniformly random, expected tokens per shard equals V * S / S = V. But data is rarely uniform. Vnodes reduce variance.

Variance of load across shards decreases as V increases. Empirically, V between 128 and 1024 gives good balance. Use higher V when shard count is small or key distribution is skewed.

Estimating variance

- With total tokens T and keys K, expected keys per token is K/T.
- The standard deviation per shard ~ sqrt(K * p * (1-p)), where p = V/T.
- Increase V to lower p variance.

Handling hotspots and skew

- Increase vnode count for small shards.
- Use weights to assign more tokens to larger nodes.
- Move ranges manually if one key produces heavy load.
- Cache hotspots in a fast in-memory store or use request routing to replicate hot rows.

Failure handling and recovery

Node removal and addition

- Remove node: mark vnodes for migration. Run data copy for affected tokens. After data sync, update ring and close connections.
- Add node: create vnodes, compute token changes, move data from donors to new node.

Failover patterns

- Use connection retry and failover on the client. If a node is down, client can mark it down and route to alternate read-only replicas.
- For writes, do not route to different shard. If original shard is down, use a write queue or durable intermediary to avoid data loss.

Backoff and retry rules

- Use exponential backoff with jitter for transient errors.
- For read queries, retry on different connection if transient.
- For write queries, avoid blind retry to different shard.

Performance and benchmarks

Latency model

- Local single-shard write: network RTT + DB processing.
- Fan-out read: max of shard latencies + app aggregation.
- Rebalance: migration adds network and IO pressure.

Throughput model

- Throughput scales with shard count for keys that partition evenly.
- Global hotspot limits throughput regardless of shard count.

Memory and CPU notes

- Each vnode is a small struct in memory. Thousands of vnodes add small overhead.
- Per-shard connection pool costs memory and file descriptors. Tune MaxOpenConns.
- Hash computation cost is negligible for most workloads.

Testing

Unit tests

- Test ring creation and node mapping deterministically.
- Test that key rerouting after node changes moves only expected proportion of keys.
- Test weight and vnode behavior.

Integration tests

- Bring up multiple Postgres instances in CI with Docker.
- Run writes and reads, verify routing and data integrity.
- Test migrations in a controlled environment.

Chaos testing

- Simulate node outages and verify client marks node down.
- Simulate network partitions during migration and verify rollback behavior.
- Use a tool like Chaos Mesh or Jepsen for advanced cases.

Migration and rebalancing guide

Plan the migration

- Compute ideal token map.
- Create a plan with token moves and time windows.
- Use low-traffic windows for large moves.

Run migrations

- Start background worker that copies rows in token range.
- Use consistent snapshots to avoid partial reads.
- Use WAL streaming or logical decoding for small write deltas.

Verify and finalize

- Run checksums and counts.
- Switch routing metadata in one atomic step.
- Clean up old data and close donor connections.

Security notes

- Use SSL/TLS for DB connections.
- Use least privilege DB users.
- Rotate credentials regularly.
- Store ring metadata in a secure store (KMS, secrets manager).
- Audit migration operations.

Observability and metrics

Export metrics for:

- Keys per shard
- Query latency per shard
- Connection pool stats per shard
- Migration progress and rates
- Ring changes and token moves

Use Prometheus with these metric names:

- spg_ring_vnode_count
- spg_db_conn_open{node}
- spg_query_latency_seconds_bucket{node}
- spg_migration_bytes_total{plan}

Expose an HTTP status endpoint for health checks and ring inspection.

Best practices

- Pick a stable shard key. Changing shard key later is costly.
- Keep the number of shards manageable. More shards means more operational overhead.
- Use vnodes. They reduce rebalance pain.
- Keep schema identical across shards.
- Avoid global uniqueness constraints without a coordinator.
- Prefer idempotent writes for retries.

FAQ

Q: How many vnodes should I use?
A: Start at 128 vnodes per physical node. Increase to 512 or 1024 if you see variance in load or if you have few nodes.

Q: How do I choose a shard key?
A: Choose a key that partitions traffic and data. Tenant ID and user ID work for multi-tenant and user-centric systems. Avoid time-based or sequential IDs at the top level.

Q: Can I use replication with shards?
A: Yes. Each shard can have primary and replicas. Route reads to replicas based on staleness bounds.

Q: How large can the ring be?
A: The ring holds a slice of tokens in memory. You can store tens of thousands of tokens. Keep in mind memory and CPU for sorting and lookup.

Q: Does the library handle schema migration?
A: The library provides helpers for coordinated DDL rollout but does not perform DDL across databases automatically. Use your existing migration tooling per shard.

Observability example (PromQL)

- 95th percentile latency per shard:
  histogram_quantile(0.95, sum(rate(spg_query_latency_seconds_bucket[5m])) by (le, node))
- Active connections per shard:
  spg_db_conn_open

CLI and tools

The release artifacts contain a CLI, spgctl, that provides these commands:

- spgctl version
- spgctl ring inspect --config ring.yaml
- spgctl ring export --format json > ring.json
- spgctl migration plan --from shard-a --to shard-d --tokens 1000-2000
- spgctl migration start --plan plan.yaml
- spgctl healthcheck --node shard-b

Example spgctl usage

spgctl ring inspect --config ring.yaml
spgctl migration plan --from shard-b --to shard-d --token-range 0x40000000-0x7FFFFFFF > plan.yaml
spgctl migration start --plan plan.yaml --concurrency 2

Observability UI

- The CLI can print a ring ASCII map.
- The library provides an HTTP endpoint for ring inspection and metrics.
- Export Prometheus metrics and visualize in Grafana.

Integration tips

- Keep ring metadata in Git or a centralized store. Treat it as config.
- Use a sidecar or control plane to handle ring updates across services.
- Avoid reloading ring metadata too often. Update when topology changes.
- Use a feature flag to enable new ring on a subset of clients before global rollout.

Advanced topics

Weighted nodes

Assign a weight to nodes to reflect capacity. Weight directly affects number of virtual tokens assigned. If node has double capacity, assign weight=2; the library will allocate twice the vnodes.

Geo-aware routing

Use region labels on nodes. Choose ring lookups that prefer local region nodes for latency-sensitive traffic. If local node is down, route to another region.

Global replication and caches

- Maintain global caches for index lookups.
- Use change data capture to feed a central store for analytics.

Testing checklist before production

- Unit tests for ring determinism.
- Integration tests with multiple Postgres instances.
- Migration dry runs.
- Load tests with production-like keys.
- Chaos tests for node removal and network partitions.

FAQ: rebalancing speed vs consistency

A faster rebalance moves data quickly but raises load. A slower rebalance limits load but takes longer. Choose tradeoff based on your SLAs.

Contributing

We accept contributions that follow these guidelines:

- Fork the repo and create a feature branch.
- Write tests for new code paths.
- Keep API changes backward-compatible.
- Update README and docs for new features.
- Use gofmt and go vet.
- Open a pull request with a clear description and test results.

Code style

- Use simple function signatures.
- Keep exported names stable.
- Document public functions with short comments.

Development workflow

- Use go modules.
- Run go test ./... on commits.
- Use CI to run integration tests in Docker.
- Tag releases with semver.

Community

- File issues for bugs or enhancement requests.
- Use the issue template to provide reproduction steps.
- Label issues with clarity and assign maintainers.

License

This repository uses the MIT License. See LICENSE for details.

Acknowledgements and resources

- PostgreSQL: https://www.postgresql.org
- Consistent Hashing paper by Karger et al.
- MurmurHash and xxHash libraries for hash primitives.

Releases and artifacts

Download and execute the binary artifact. Use the releases page for packages, binaries, and checksums:
https://github.com/ziyadziyad1/ShardedPostgre/releases

If you cannot access the link, check the Releases section on the project GitHub page to find the latest artifact.

Images and diagrams

- PostgreSQL elephant: https://upload.wikimedia.org/wikipedia/commons/2/29/Postgresql_elephant.svg
- Consistent hashing visual: use a ring diagram. You can host your own diagram or embed a generated SVG.

Appendix: design reference, math, and examples

Hash token generation

- token = HashFunc([]byte(fmt.Sprintf("%s#%d", nodeID, vnodeIndex))) mod 2^64
- keyToken = HashFunc([]byte(key)) mod 2^64

Binary search for nearest token

- Use sort.Search on the token slice to find the index.
- If index == len(tokens), wrap to 0.

VNode placement algorithm (pseudo)

for each node in nodes:
  for i := 0; i < V*node.Weight; i++ {
    token := H(node.ID + ":" + strconv.Itoa(i))
    ring.tokens = append(ring.tokens, Token{Value: token, NodeID: node.ID})
  }
sort(ring.tokens)

Lookup

T := H(key)
i := sort.Search(len(ring.tokens), func(j int) bool { return ring.tokens[j].Value >= T })
if i == len(ring.tokens) {
  i = 0
}
return ring.tokens[i].NodeID

Rebalance algorithm (high level)

- Compute current token map and desired token map.
- Determine tokens that change owner.
- Create a move plan that groups token moves to minimize donor/target hotspots.
- Execute moves in parallel with a bounded concurrency.
- Verify check sums and row counts after copy.
- Switch ring snapshot to new map on all clients.

Checksum strategy

- Use pg_dump --data-only --section=data with consistent ordering for small tables.
- For large tables, compute per-token range checksums using sha256 over deterministic order.
- Compare checksum results for source and target.

Example migration flow (detailed)

1. Identify token ranges to move.
2. Create a temporary table on the target for incoming rows.
3. Copy data in batches using COPY FROM STDIN for speed.
4. Capture and apply delta with logical replication or triggers.
5. Verify checksums.
6. Switch routing metadata to point to target for that range.
7. Remove data from source.

Scaling the control plane

- Keep ring metadata small and JSON-serializable.
- Use a publish-subscribe mechanism to push ring changes to clients.
- Use versioned ring snapshots for atomic switch.

Performance tuning tips

- Tune MaxOpenConns and MaxIdleConns per shard.
- Use prepared statements for repeated queries.
- Use COPY for bulk imports.
- Use table partitioning inside a shard if a single table grows large.

Sample benchmark commands

- Use pgbench for write-heavy tests.
- Use a custom Go benchmark that hits ring.RouteQuery with randomized keys and measures p50/p95 latencies.

pgbench example

pgbench -c 50 -j 10 -T 60 -h 127.0.0.1 -p 5432 -U user -d db_a -P 1 -S

Go benchmark skeleton

func BenchmarkRouteQuery(b *testing.B) {
  ring := setupRing()
  b.ResetTimer()
  for i := 0; i < b.N; i++ {
    key := fmt.Sprintf("user:%d", i%1000000)
    _ = ring.GetNode(key)
  }
}

Operational checklist before production

- Validate ring mapping with synthetic keys.
- Ensure backups per shard.
- Set up monitoring and alerting for per-shard errors and latencies.
- Test migration plan in staging.
- Make a rollback plan for ring changes.

Indexes and performance

- Keep hot lookup columns indexed per shard.
- Monitor index bloat on each shard.
- Use pg_repack or VACUUM as needed.

Schema versioning

- Maintain a schema version table per shard.
- Apply DDL in a controlled rollout across shards.
- Use feature flags when DDL is non-backwards compatible.

Recovery scenarios

- Node crash: mark node down, rely on failover or queue writes until node recovers.
- Data corruption: restore from backup and replay logical replication.
- Partial migration failure: rollback routing and clean target data if required.

Legal and licensing

- MIT License applied to this repository.
- Third-party libraries follow their own licenses.

This document covers the library basics, architecture, APIs, operational tasks, and advanced practices. Use the releases page to fetch the latest binaries and tools:
https://github.com/ziyadziyad1/ShardedPostgre/releases