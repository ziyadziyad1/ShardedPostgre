# ShardedPostgre

A PostgreSQL sharding library implementing consistent hashing with virtual nodes for horizontal scaling.

## Configuration

The library requires a JSON configuration file that specifies the shards and virtual nodes settings.

### Configuration File Structure
```json
{
  "ConfigShards": [
    {
      "name": "shard1",
      "Dns": "postgres://username:password@localhost:5432/dbname1"
    },
    {
      "name": "shard2",
      "Dns": "postgres://username:password@localhost:5433/dbname2"
    }
  ],
  "Vnodes": 50
}
```
## Configuration Management Viewing Configuration
Viewing Configuration
```go
// Print current configuration
err := sharding.ViewConfig("config.json")
```

Modifying Configuration
```go
// Clear all shards (use with caution)
err := sharding.ClearConfig("config.json")


// Modify virtual nodes count (don't use at runtime)
err := sharding.ModifyConfigVnodes(100, "config.json")
```

!If you want you can still modify the config manually!

### Configuration Fields
- ConfigShards : Array of shard configurations
  - name : Unique identifier for the shard
  - Dns : PostgreSQL connection string (pgx format)
- Vnodes : Number of virtual nodes per shard (recommended: 50-200)

## Usage

### Initializing the Ring Manager
```go
import "github.com/Mister-dev-oss/ShardedPostgre/sharding"

// Initialize the ring manager with your config file
rm, err := sharding.InitRingManager("config.json")
if err != nil {
    log.Fatal(err)
}
```
### Executing Queries
```go
// Query that returns multiple rows
rows, err := rm.Query(types.Query{
    Key: "user123",  // Sharding key
    Sql: "SELECT * FROM users WHERE department = $1",
    Args: []interface{}{"engineering"},
})

// Query that returns a single row
row, err := rm.QueryRow(types.Query{
    Key: "user456",
    Sql: "SELECT name FROM users WHERE id = $1",
    Args: []interface{}{1},
})

// Execute commands (INSERT, UPDATE, DELETE)
result, err := rm.Exec(types.Query{
    Key: "user789",
    Sql: "INSERT INTO users (name, email) VALUES ($1, $2)",
    Args: []interface{}{"John", "john@example.com"},
})
```

### Managing Shards at Runtime
```go
// Add a new shard
err := rm.AddShard("shard3", "postgres://username:password@localhost:5434/dbname3")

// Remove a shard
err := rm.RemoveShard("shard3")
```

## Important Notes
1. Configuration File Management :
   
   - Keep config.json in a secure location
   - Backup before modifications
   - Use RingManager methods for runtime changes
   - Direct file modifications only during setup
2. Connection Strings : Use pgx connection string format:
   
   ```text
   postgres://username:password@host:port/dbname?param1=value1&param2=value2
   ```
3. Sharding Keys : Choose keys that provide even distribution across shards
   
   - User IDs
   - Tenant IDs
   - Hash of composite keys
4. Virtual Nodes :
   
   - Higher number = better distribution but more memory usage
   - Lower number = less memory but potential hotspots
   - Recommended: start with 50 and adjust based on needs
5. Runtime Operations :
   
   - Use only RingManager methods for shard operations
   - Avoid modifying the config file directly
   - Don't change Vnodes count during runtime

## Best Practices
1. Configuration Setup :
   
   - Start with a minimal configuration
   - Test configuration before deployment
   - Keep connection strings secure
   - Document shard allocation
2. Query Design :
   
   - Include sharding key in all queries
   - Keep related data in the same shard
   - Design schema with sharding in mind
3. Monitoring :
   
   - Regularly check shard distribution
   - Monitor query performance across shards
   - Keep shard sizes balanced

## Limitations
- No cross-shard transactions
- No automatic rebalancing
- No built-in backup/restore functionality
- Configuration changes require careful planning

## Thread Safety
The library is thread-safe and can handle concurrent operations:

- Multiple simultaneous queries
- Runtime shard additions/removals
- Configuration updates

## Error Handling
Always check returned errors, especially for:

- Ring initialization
- Shard operations
- Query execution
- Configuration changes

## Example Configuration Setup
1. Create initial config.json:
```json
{
  "ConfigShards": [
    {
      "name": "shard1",
      "Dns": "postgres://user:pass@localhost:5432/db1"
    }
  ],
  "Vnodes": 50
}
 ```

2. Initialize and verify:
```go
if err := sharding.ViewConfig("config.json"); err != nil {
    log.Fatal(err)
}

rm, err := sharding.InitRingManager("config.json")
if err != nil {
    log.Fatal(err)
}
 ```

3. Start using the ring manager for all operations.