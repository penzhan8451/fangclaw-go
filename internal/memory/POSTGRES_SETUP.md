# PostgreSQL + pgvector Setup Guide for FangClaw-Go

This guide explains how to switch from SQLite to PostgreSQL with pgvector support.

## Prerequisites

1. PostgreSQL 14+ installed
2. pgvector extension installed

### Installing pgvector

**macOS (Homebrew):**
```bash
brew install pgvector
```

**Ubuntu/Debian:**
```bash
sudo apt-get install postgresql-14-pgvector
```

**Docker:**
```bash
docker run -d \
  --name fangclaw-postgres \
  -e POSTGRES_USER=fangclaw \
  -e POSTGRES_PASSWORD=yourpassword \
  -e POSTGRES_DB=fangclaw \
  -p 5432:5432 \
  ankane/pgvector:latest
```

## Configuration

### Environment Variables

Set the following environment variables to enable PostgreSQL:

```bash
# Required
export FANGCLAW_STORAGE_TYPE=postgres
export FANGCLAW_POSTGRES_HOST=localhost
export FANGCLAW_POSTGRES_PORT=5432
export FANGCLAW_POSTGRES_DATABASE=fangclaw
export FANGCLAW_POSTGRES_USER=fangclaw
export FANGCLAW_POSTGRES_PASSWORD=yourpassword
export FANGCLAW_POSTGRES_SSLMODE=disable

# Optional (defaults shown)
# export FANGCLAW_POSTGRES_SSLMODE=disable  # Options: disable, require, verify-ca, verify-full
```

### Or use a single DATABASE_URL

```bash
export FANGCLAW_DATABASE_URL="postgres://fangclaw:yourpassword@localhost:5432/fangclaw?sslmode=disable"
```

## Switching from SQLite to PostgreSQL

### Step 1: Backup SQLite Data (Optional)

```bash
cp ~/.fangclaw/fangclaw.db ~/.fangclaw/fangclaw.db.backup
```

### Step 2: Create PostgreSQL Database

```bash
# Connect to PostgreSQL
psql -U postgres

# Create database and user
CREATE DATABASE fangclaw;
CREATE USER fangclaw WITH ENCRYPTED PASSWORD 'yourpassword';
GRANT ALL PRIVILEGES ON DATABASE fangclaw TO fangclaw;

# Connect to the new database and enable pgvector
\c fangclaw
CREATE EXTENSION IF NOT EXISTS vector;
```

### Step 3: Configure FangClaw

Add to your `.env` file or environment:

```bash
FANGCLAW_STORAGE_TYPE=postgres
FANGCLAW_POSTGRES_HOST=localhost
FANGCLAW_POSTGRES_PORT=5432
FANGCLAW_POSTGRES_DATABASE=fangclaw
FANGCLAW_POSTGRES_USER=fangclaw
FANGCLAW_POSTGRES_PASSWORD=yourpassword
FANGCLAW_POSTGRES_SSLMODE=disable
```

### Step 4: Start FangClaw

```bash
./fangclaw-go
```

The application will automatically:
1. Connect to PostgreSQL
2. Run migrations to create tables
3. Enable pgvector extension
4. Create indexes

## Migration from SQLite to PostgreSQL

### Option 1: Fresh Start (Recommended for Development)

Simply switch to PostgreSQL and start fresh. No data migration needed.

### Option 2: Data Migration (Production)

To migrate existing data from SQLite to PostgreSQL:

```bash
# 1. Export SQLite data
sqlite3 ~/.fangclaw/fangclaw.db .dump > fangclaw_backup.sql

# 2. Convert SQL syntax (SQLite -> PostgreSQL)
# - Replace AUTOINCREMENT with SERIAL
# - Replace ? with $1, $2, etc.
# - Replace datetime('now') with NOW()
# - etc.

# 3. Import to PostgreSQL
psql -U fangclaw -d fangclaw -f fangclaw_backup.sql
```

Note: A proper migration tool would be needed for production use.

## Verification

### Check Connection

```bash
# Test PostgreSQL connection
psql -U fangclaw -d fangclaw -c "SELECT version();"

# Check if pgvector is installed
psql -U fangclaw -d fangclaw -c "SELECT * FROM pg_extension WHERE extname = 'vector';"
```

### Check FangClaw Tables

```sql
\dt
```

Expected tables:
- agents
- sessions
- memory
- kv_store
- memories (with vector column)
- audit
- triggers
- trigger_history
- workflows
- cron_jobs

### Test Vector Search

```sql
-- Check if vector index exists
SELECT * FROM pg_indexes WHERE tablename = 'memories';

-- Test vector similarity search
SELECT id, content, embedding <=> '[0.1, 0.2, ...]'::vector AS distance
FROM memories
WHERE deleted = false
ORDER BY embedding <=> '[0.1, 0.2, ...]'::vector
LIMIT 10;
```

## Performance Tuning

### Connection Pool Settings

The PostgreSQL implementation uses these defaults:
- Max Open Connections: 25
- Max Idle Connections: 5
- Connection Max Lifetime: 5 minutes

Adjust in `db_postgres.go` if needed.

### Vector Index Tuning

The default IVFFlat index uses 100 lists. For larger datasets:

```sql
-- Recreate index with more lists (for 100k+ vectors)
DROP INDEX idx_memories_embedding;
CREATE INDEX idx_memories_embedding ON memories 
USING ivfflat (embedding vector_cosine_ops) WITH (lists = 1000);
```

### PostgreSQL Configuration

Recommended `postgresql.conf` settings for FangClaw:

```ini
# Memory
shared_buffers = 256MB
effective_cache_size = 1GB
work_mem = 16MB

# Connections
max_connections = 100

# WAL
wal_buffers = 16MB
```

## Troubleshooting

### Connection Errors

```
failed to ping postgres database: dial tcp localhost:5432: connect: connection refused
```

**Solution:**
- Check PostgreSQL is running: `pg_isready`
- Verify host/port in configuration
- Check firewall settings

### pgvector Not Found

```
failed to create pgvector extension: ERROR: extension "vector" does not exist
```

**Solution:**
```bash
# Install pgvector
# macOS:
brew install pgvector

# Ubuntu:
sudo apt-get install postgresql-14-pgvector

# Or use Docker image with pgvector pre-installed
docker pull ankane/pgvector
```

### SSL Mode Errors

```
failed to ping postgres database: pq: SSL is not enabled on the server
```

**Solution:**
Set `FANGCLAW_POSTGRES_SSLMODE=disable` for local development.

## Reverting to SQLite

Simply unset the PostgreSQL environment variables or set:

```bash
export FANGCLAW_STORAGE_TYPE=sqlite
```

The application will fall back to SQLite (default behavior).

## Code Integration

### Using the Storage Interface

```go
import "github.com/penzhan8451/fangclaw-go/internal/memory"

// Create storage based on configuration
config := memory.StorageConfig{
    Type: memory.StoragePostgres,
    Postgres: memory.PostgresConfig{
        Host:     "localhost",
        Port:     5432,
        Database: "fangclaw",
        User:     "fangclaw",
        Password: "password",
        SSLMode:  "disable",
    },
}

storage, err := memory.NewStorage(config)
if err != nil {
    log.Fatal(err)
}
defer storage.Close()

// Run migrations
if err := storage.Migrate(); err != nil {
    log.Fatal(err)
}

// Use storage...
```

### Using Semantic Storage with PostgreSQL

```go
// Create PostgreSQL storage
pgStorage, err := memory.NewPostgresDB(config)
if err != nil {
    log.Fatal(err)
}

// Create semantic store with vector support
semanticStore, err := memory.NewPostgresSemanticStore(pgStorage)
if err != nil {
    log.Fatal(err)
}

// Store memory with embedding
id, err := semanticStore.RememberWithEmbedding(
    agentID,
    "content",
    source,
    "scope",
    metadata,
    embedding, // []float32
)

// Search with vector similarity
results, err := semanticStore.RecallWithEmbedding(
    "query",
    10, // limit
    nil, // filter
    queryEmbedding, // []float32
)
```

## Architecture Notes

### Why PostgreSQL + pgvector?

1. **Scalability**: Better handling of large datasets (100k+ memories)
2. **Concurrent Access**: Multiple agents can read/write simultaneously
3. **Vector Search**: Native vector similarity with IVFFlat/HNSW indexes
4. **Production Ready**: Better monitoring, backup, and replication support

### Trade-offs

| Aspect | SQLite | PostgreSQL + pgvector |
|--------|--------|----------------------|
| Setup | Zero config | Requires PostgreSQL + pgvector |
| Deployment | Single binary | Needs database server |
| Concurrency | Limited | Excellent |
| Vector Search | Application layer | Database layer |
| Scalability | ~10k memories | 100k+ memories |
| Backup | File copy | pg_dump, WAL archiving |

## Future Enhancements

- [ ] Data migration tool from SQLite to PostgreSQL
- [ ] Connection pooling configuration via environment variables
- [ ] Read replicas support
- [ ] Automatic failover
- [ ] Vector index optimization based on data size
