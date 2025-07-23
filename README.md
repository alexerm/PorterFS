# PorterFS

A lightweight, single-binary S3-compatible object storage server that stores files on local POSIX filesystems.

## Features

- **S3-Compatible API**: Supports core S3 operations (GET, PUT, DELETE, ListObjects, ListObjectsV2)
- **AWS V4 Signature Authentication**: Compatible with standard S3 clients
- **Local Filesystem Storage**: Maps S3 buckets to directories on disk
- **Single Binary**: No dependencies, easy deployment
- **Cross-Platform**: Builds for macOS and Linux (AMD64/ARM64)

## Quick Start

### 1. Download Binary

Download the appropriate binary for your platform from the releases page or build from source:

```bash
# Build from source
go build -o porter ./cmd/porter

# Or use pre-built binaries
./porter-darwin-arm64  # macOS ARM64
./porter-linux-amd64   # Linux AMD64
```

### 2. Create Configuration

Create a `config.yaml` file (see [config.yaml.example](config.yaml.example)):

```yaml
server:
  address: ":9000"
  tls:
    enabled: false

storage:
  root_path: "./data"
  max_size_bytes: 107374182400  # 100GB

auth:
  access_key: "porterfs"
secret_key: "porterfs"

logging:
  level: "info"
  format: "json"
```

### 3. Start Server

```bash
./porter -config config.yaml
```

The server will start on port 9000 and create a `data/` directory for storage.

## Usage Examples

### AWS CLI

Configure AWS CLI to use PorterFS:

```bash
aws configure set aws_access_key_id porterfs
aws configure set aws_secret_access_key porterfs
aws configure set default.region us-east-1
```

Basic operations:

```bash
# Create bucket
aws --endpoint-url http://localhost:9000 s3 mb s3://my-bucket

# Upload file
aws --endpoint-url http://localhost:9000 s3 cp file.txt s3://my-bucket/

# List objects
aws --endpoint-url http://localhost:9000 s3 ls s3://my-bucket/

# Download file
aws --endpoint-url http://localhost:9000 s3 cp s3://my-bucket/file.txt downloaded.txt

# Delete object
aws --endpoint-url http://localhost:9000 s3 rm s3://my-bucket/file.txt
```

### s3cmd

Configure s3cmd:

```bash
s3cmd --configure
# Use these settings:
# Access Key: porterfs
# Secret Key: porterfs
# Default Region: us-east-1
# S3 Endpoint: localhost:9000
# Use HTTPS: No
```

### rclone

Add to rclone config:

```ini
[porter]
type = s3
provider = Other
access_key_id = porterfs
secret_access_key = porterfs
endpoint = http://localhost:9000
```

## API Compatibility

### Supported Operations

- ✅ ListBuckets
- ✅ CreateBucket / DeleteBucket
- ✅ ListObjects (v1)
- ✅ ListObjectsV2
- ✅ GetObject
- ✅ PutObject
- ✅ DeleteObject
- ✅ HeadObject

### Planned (v0.3+)

- ⏳ Multipart Upload (≥5GB files)
- ⏳ HTTP Range Requests
- ⏳ Object Versioning
- ⏳ Server-Side Encryption

## Configuration

### Server Options

- `address`: Server bind address (default: ":9000")
- `tls.enabled`: Enable HTTPS (default: false)
- `tls.cert_file`: TLS certificate file path
- `tls.key_file`: TLS private key file path

### Storage Options

- `root_path`: Root directory for object storage (default: "./data")
- `max_size_bytes`: Maximum storage size in bytes (default: 100GB)

### Authentication

- `access_key`: S3 access key (default: "porterfs")
- `secret_key`: S3 secret key (default: "porterfs")

### Logging

- `level`: Log level - debug, info, warn, error (default: "info")
- `format`: Log format - json, text (default: "json")

## Development

### Building

```bash
# Local build
go build ./cmd/porter

# Cross-platform builds
make build-all

# Run tests
go test ./...
```

### Project Structure

```
├── cmd/porter/          # Main application
├── internal/
│   ├── auth/           # AWS V4 signature authentication
│   ├── config/         # Configuration management
│   ├── handlers/       # HTTP request handlers
│   ├── server/         # HTTP server setup
│   └── storage/        # Storage interface and local implementation
└── docs/               # Documentation
```

## Roadmap

### v0.1 ✅
- Basic S3 API (GET, ListObjects)
- AWS V4 authentication
- Local filesystem storage

### v0.2 ✅
- PUT/DELETE operations
- Docker image
- Comprehensive tests

### v0.3 (In Progress)
- Multipart uploads (≥5GB)
- HTTP range requests
- CI/CD pipeline

### v0.4+
- Web UI
- OpenID Connect auth
- Per-bucket policies
- Server-side encryption

## License

Apache 2.0 License - see [LICENSE](LICENSE) file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## Support

- GitHub Issues: Report bugs and feature requests
- Discussions: General questions and community support