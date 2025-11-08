# CaskOS - Mini S3-style Object Storage System

CaskOS is a production-quality, distributed object storage system written in Go, inspired by Amazon S3 and Ceph RGW. It provides object storage with replication, consistent hashing, and self-healing capabilities.

## Features

- **Object Storage**: Store and retrieve binary objects with unique IDs
- **Replication**: Automatic replication across multiple storage nodes (default: 2 replicas)
- **Consistent Hashing**: Efficient node selection using a hash ring algorithm
- **Self-Healing**: Automatic detection and repair of missing replicas
- **Metadata Management**: JSON-based metadata store for object information
- **RESTful API**: Simple HTTP API for upload, download, and metadata operations
- **Web UI**: Simple, modern web interface for file uploads
- **Docker Support**: Ready-to-run containerized deployment

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      API Gateway Server                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │  POST    │  │   GET    │  │   GET    │  │   GET    │  │
│  │ /upload  │  │/object/{id}│ │/metadata/{id}│ /health │  │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘  │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
        ┌───────────────────────────────────────┐
        │        Storage Manager                │
        │  - Replication Coordination            │
        │  - Node Selection (Hash Ring)         │
        │  - Self-Healing Logic                  │
        └───────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│ Storage Node │   │ Storage Node │   │ Storage Node │
│    (node1)   │   │    (node2)   │   │    (node3)   │
│              │   │              │   │              │
│ /data/node1/ │   │ /data/node2/ │   │ /data/node3/ │
│  ab/cd/obj   │   │  ab/cd/obj   │   │  ab/cd/obj   │
└──────────────┘   └──────────────┘   └──────────────┘
        │                   │                   │
        └───────────────────┼───────────────────┘
                            │
                            ▼
                ┌──────────────────────┐
                │   Metadata Store     │
                │  (JSON files)        │
                │  /metadata/{id}.json │
                └──────────────────────┘
                            │
                            ▼
                ┌──────────────────────┐
                │   Hash Ring          │
                │  (Consistent Hashing) │
                │  - Virtual Nodes      │
                │  - Node Selection     │
                └──────────────────────┘
```

### Component Overview

1. **API Server** (`internal/api`): HTTP server handling upload/download requests
2. **Storage Manager** (`internal/storage`): Coordinates replication and node selection
3. **Storage Nodes** (`internal/storage`): Individual storage directories representing disks
4. **Metadata Store** (`internal/metadata`): JSON-based metadata persistence
5. **Hash Ring** (`internal/hashring`): Consistent hashing for node assignment

### Consistent Hashing

CaskOS uses consistent hashing to distribute objects across storage nodes:

- Each physical node has multiple **virtual nodes** (default: 150) on the hash ring
- Objects are assigned to nodes based on their SHA256 hash
- Adding/removing nodes only affects a small portion of objects (minimal data shuffling)
- The hash ring ensures even distribution and efficient lookup

**How it works:**
1. Object ID is hashed to a position on the ring
2. The system finds the first node clockwise from that position
3. For replication, it selects N distinct nodes following the ring
4. This ensures consistent placement even as nodes are added/removed

## Installation

### Prerequisites

- Go 1.22 or later
- Docker and Docker Compose (for containerized deployment)

### Build from Source

```bash
# Clone the repository
git clone <repository-url>
cd CaskOS

# Build the binary
go build -o caskos ./cmd/caskos

# Run locally
./caskos -port 8080 -data-dir ./data -metadata-dir ./metadata -nodes 3 -replication 2
```

## Usage

### Running Locally

```bash
# Start the server with default settings
./caskos

# Custom configuration
./caskos -port 8080 \
         -data-dir ./data \
         -metadata-dir ./metadata \
         -nodes 3 \
         -replication 2 \
         -virtual-nodes 150
```

Once the server is running, open your browser to `http://localhost:8080` to access the web UI.

**Command-line flags:**
- `-port`: HTTP server port (default: 8080)
- `-data-dir`: Base directory for storage nodes (default: ./data)
- `-metadata-dir`: Directory for metadata storage (default: ./metadata)
- `-nodes`: Number of storage nodes (default: 3)
- `-replication`: Replication factor (default: 2)
- `-virtual-nodes`: Virtual nodes per physical node (default: 150)

### Running with Docker Compose

```bash
# Build and start the service
docker-compose up -d

# View logs
docker-compose logs -f

# Stop the service
docker-compose down
```

The service will be available at `http://localhost:8080`.

## Web UI

CaskOS includes a simple, modern web interface for uploading files. Simply navigate to `http://localhost:8080` in your browser after starting the server.

**Features:**
- Drag-and-drop file selection
- Real-time upload progress
- Display of object ID, metadata, and download links
- Copy-to-clipboard for object IDs
- Direct links to download files and view metadata

## API Usage

### Upload an Object

```bash
curl -X POST http://localhost:8080/upload \
  -F "file=@/path/to/your/file.jpg" \
  -H "Content-Type: multipart/form-data"
```

**Response:**
```json
{
  "id": "a1b2c3d4e5f6...",
  "size": 12345,
  "content_type": "image/jpeg",
  "created_at": "2024-01-15T10:30:00Z",
  "replicas": ["node1", "node2"]
}
```

### Retrieve an Object

```bash
curl http://localhost:8080/object/{object-id} --output downloaded-file.jpg
```

### Get Object Metadata

```bash
curl http://localhost:8080/metadata/{object-id}
```

**Response:**
```json
{
  "id": "a1b2c3d4e5f6...",
  "size": 12345,
  "content_type": "image/jpeg",
  "created_at": "2024-01-15T10:30:00Z",
  "replicas": ["node1", "node2"]
}
```

### Health Check

```bash
curl http://localhost:8080/health
```

Returns `OK` if the service is healthy.

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/` | Web UI (HTML interface) |
| POST | `/upload` | Upload a file (multipart/form-data) |
| GET | `/object/{id}` | Download an object by ID |
| GET | `/metadata/{id}` | Get object metadata |
| GET | `/health` | Health check |
| GET | `/static/*` | Static files (CSS, JS) |

## Self-Healing

CaskOS includes automatic self-healing capabilities:

1. **Detection**: When retrieving metadata or objects, the system checks replica availability
2. **Repair**: If replicas are missing (below replication factor), the system:
   - Retrieves the object from an available replica
   - Replicates it to nodes that should have it (according to hash ring)
   - Updates metadata with the new replica list
3. **Background Process**: Self-healing runs asynchronously to avoid blocking API requests

## Testing

Run the test suite:

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test
go test -v ./internal/hashring
```

## Project Structure

```
CaskOS/
├── cmd/
│   └── caskos/
│       └── main.go              # Application entry point
├── internal/
│   ├── api/
│   │   └── server.go            # HTTP API server
│   ├── storage/
│   │   ├── node.go              # Storage node implementation
│   │   └── manager.go          # Storage manager with replication
│   ├── metadata/
│   │   └── store.go             # Metadata store (JSON-based)
│   └── hashring/
│       └── hashring.go          # Consistent hashing implementation
├── web/
│   └── static/
│       ├── index.html           # Web UI HTML
│       ├── style.css            # Web UI styles
│       └── app.js               # Web UI JavaScript
├── test/
│   └── integration_test.go       # Integration tests
├── Dockerfile                   # Docker build file
├── docker-compose.yml           # Docker Compose configuration
├── go.mod                       # Go module definition
└── README.md                    # This file
```

## Design Decisions

### Why JSON Files for Metadata?

- **Simplicity**: No external dependencies (BoltDB/BadgerDB would require additional packages)
- **Debuggability**: Easy to inspect and modify metadata files
- **Portability**: Works on any filesystem
- **Performance**: Sufficient for most use cases; can be swapped for a database later

### Why Consistent Hashing?

- **Scalability**: Adding/removing nodes doesn't require full data rebalancing
- **Distribution**: Ensures even distribution of objects across nodes
- **Predictability**: Same object always maps to the same nodes (when nodes are stable)

### Why SHA256 for Object IDs?

- **Content-addressable**: Same content = same ID (deduplication)
- **Deterministic**: No need for external ID generation
- **Collision-resistant**: Extremely low probability of collisions

## Performance Considerations

- **File Size**: Currently reads entire files into memory for replication. For very large files (>100MB), consider streaming replication.
- **Concurrency**: Uses Go's standard library with goroutines for concurrent operations.
- **Metadata**: JSON file-based storage is fast for small to medium deployments. For high-throughput scenarios, consider migrating to BoltDB or BadgerDB.

## Limitations and Future Enhancements

### Current Limitations

- Single-node deployment (all storage nodes on one machine)
- No authentication/authorization
- No object versioning

### Potential Enhancements

- [ ] Multi-machine distributed deployment
- [ ] Basic authentication with API keys
- [ ] Object versioning support
- [x] Web UI for file uploads
- [ ] Streaming replication for large files
- [ ] Metrics and monitoring endpoints
- [ ] Object expiration/TTL
- [ ] Range requests for partial downloads
- [ ] File browser/list view in web UI

## License

This project is provided as-is for educational and development purposes.

## Contributing

Contributions are welcome! Please ensure:
- Code follows Go best practices
- Tests are included for new features
- Documentation is updated

## Author

Built as a production-quality distributed systems project demonstrating:
- Consistent hashing
- Replication strategies
- Self-healing systems
- Clean Go architecture

