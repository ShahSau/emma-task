# Bulk Import/Export API

A high-performance, production-ready API for bulk importing and exporting users, articles, and comments. Built with Go, Gin, PostgreSQL, and S3 (LocalStack), this system efficiently handles datasets up to 1,000,000+ records with streaming processing and async job management.

---

## 🚀 Features

### Core Capabilities
- **Multi-format Support**: CSV, NDJSON, and JSON array
- **Async Job Processing**: Background workers handle large imports/exports
- **Streaming Architecture**: O(1) memory usage for unlimited dataset sizes
- **Robust Validation**: Per-record validation with detailed error reporting
- **S3 Storage**: Presigned URLs for secure file downloads
- **Real-time Progress**: Track job status and progress in real-time
- **Idempotency**: Prevent duplicate imports with idempotency keys
- **Filter Support**: Export data with custom filters

### Supported Resources
- **Users**: Email, name, role, UUID
- **Articles**: Title, slug, body, tags, author relationships
- **Comments**: Body, article/user references

### Performance Characteristics
- **Import Speed**: 5,000+ rows/second (NDJSON)
- **Export Speed**: 6,500+ rows/second (streaming)
- **Memory Usage**: O(1) constant (streaming architecture)
- **Batch Size**: 1,000 records per database transaction
- **Max Dataset**: 1,000,000+ records per job

---

## 📋 Table of Contents

1. [Quick Start](#quick-start)
2. [Architecture](#architecture)
3. [API Documentation](#api-documentation)
4. [Environment Configuration](#environment-configuration)
5. [Development](#development)
6. [Testing](#testing)
7. [Deployment](#deployment)
8. [Troubleshooting](#troubleshooting)

---

## 🏁 Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.21+ (for local development)
- Git

### Installation

1. **Clone the repository**
```bash
git clone <your-repo-url>
cd golang-gin-realworld-example-app
```

2. **Start the application**
```bash
docker-compose up -d
```

4. **Verify the application is running**
```bash
curl http://localhost:8080/api/ping
# Expected: {"message":"pong"}
```

### Test Import
```bash
# Import users from CSV
curl -X POST http://localhost:8080/v1/imports \
  -F "file=@users_huge.csv" \
  -F "resource=users"

# Response:
# {
#   "job_id": "abc-123-def",
#   "status": "PENDING",
#   "created_at": "2026-02-05T12:00:00Z"
# }

# Check import status
curl http://localhost:8080/v1/imports/abc-123-def

# Download error report (if any errors occurred)
curl http://localhost:8080/v1/imports/abc-123-def/errors -o errors.ndjson
```

### Test Export
```bash
# Synchronous streaming export (small datasets)
curl "http://localhost:8080/v1/exports?resource=users&format=ndjson" > users.ndjson

# Async export job (large datasets)
curl -X POST http://localhost:8080/v1/exports \
  -H "Content-Type: application/json" \
  -d '{
    "resource": "articles",
    "format": "ndjson",
    "filters": {
      "author": "johndoe"
    }
  }'

# Check export status and get download URL
curl http://localhost:8080/v1/jobs/abc-123-def
```

---

## 🏗️ Architecture

### System Overview
```
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│   Client    │─────▶│  Gin Router  │─────▶│  PostgreSQL │
│  (curl/app) │      │  (REST API)  │      │  (Database) │
└─────────────┘      └──────────────┘      └─────────────┘
                            │
                            │
                     ┌──────▼────────┐
                     │  Job Queue    │
                     │  (Database)   │
                     └──────┬────────┘
                            │
                     ┌──────▼────────┐
                     │ Worker Pool   │
                     │ (Background)  │
                     └──────┬────────┘
                            │
                     ┌──────▼────────┐
                     │  S3/LocalStack│
                     │  (File Store) │
                     └───────────────┘
```

### Key Design Decisions

**1. Streaming Architecture**
- Uses `io.Pipe` for zero-copy streaming
- No temporary files on disk
- Constant memory usage regardless of dataset size

**2. Database Polling for Jobs**
- Uses PostgreSQL `FOR UPDATE SKIP LOCKED`
- Restart-safe job queue
- Supports multiple worker instances
- No in-memory queues that lose data on restart

**3. S3 for File Storage**
- All imports/exports stored in S3 (LocalStack for dev)
- Presigned URLs for secure, time-limited downloads
- No local filesystem dependencies

**4. Soft Validation**
- Validates each record independently
- Continues processing on validation errors
- Generates detailed error reports
- Achieves high throughput even with bad data

**5. UUID Primary Keys**
- Distributed-systems friendly
- No auto-increment conflicts
- Supports data import/export across systems

---

## 📚 API Documentation

For detailed API documentation with request/response examples, see **[API.md](./API.md)**.

### Quick Reference

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/v1/imports` | POST | Create import job |
| `/v1/imports/:id` | GET | Get import status |
| `/v1/imports/:id/errors` | GET | Download error report |
| `/v1/exports` | GET | Sync streaming export |
| `/v1/exports` | POST | Create async export job |
| `/v1/exports/:job_id` | GET | Get job status (import or export) |

---

## 💻 Development

### Local Development Setup

1. **Install dependencies**
```bash
go mod download
```

2. **Run PostgreSQL and LocalStack**
```bash
docker-compose up -d postgres localstack
```

3. **Set environment variables**
```bash
export DB_HOST=localhost
export AWS_ENDPOINT=http://localhost:4566
export S3_BUCKET=bulk-imports
# ... other vars from .env.example
```

4. **Run the application**
```bash
go run main.go
```

5. **Run tests**
```bash
go test ./... -v
```

### Project Structure
```
.
├── main.go                 # Application entry point
├── docker-compose.yml      # Docker configuration
├── .env.example           # Environment template
│
├── common/                # Shared utilities
│   ├── database.go        # PostgreSQL connection
│   └── s3.go             # S3 client & presigned URLs
│
├── jobs/                  # Bulk operations module
│   ├── models.go         # Job model & database operations
│   ├── routers.go        # Import route handlers
│   ├── core/
│   │   ├── importer.go   # Import processing logic
│   │   └── exporter.go   # Export processing logic
│   └── worker/
│       └── dispatcher.go # Background job worker
│       └── processor.go # Background job worker
│
├── routers/              # Additional route handlers
│   ├── exports.go       # Export route handlers
│   └── jobs.go          # Job status handlers
│
├── users/               # User domain
│   └── models.go
│
└── articles/            # Article domain
    └── models.go
```
---

## 🧪 Testing

### Unit Tests
```bash
# Run all tests
go test ./... -v

# Run specific package tests
go test ./jobs/core -v

# Run with coverage
go test ./... -cover
```

### Integration Tests
```bash
# Test complete import flow
./scripts/test_import.sh

# Test complete export flow
./scripts/test_export.sh
```

### Manual Testing with Sample Data

Sample test files are provided in the repository:
```bash
# Import 10,000 users (CSV)
curl -X POST http://localhost:8080/v1/imports \
  -F "file=@users_huge.csv" \
  -F "resource=users"

# Import 10,000 articles (NDJSON)
curl -X POST http://localhost:8080/v1/imports \
  -F "file=@articles_huge.ndjson" \
  -F "resource=articles"

# Import 50,000 comments (NDJSON)
curl -X POST http://localhost:8080/v1/imports \
  -F "file=@comments_huge.ndjson" \
  -F "resource=comments"
```

### Docker Production Build
```bash
# Build production image
docker build -t bulk-api:latest .

# Run with production env
docker run -d \
  --name bulk-api \
  --env-file .env.production \
  -p 8080:8080 \
  bulk-api:latest
```

### Scaling Workers

To handle higher throughput, run multiple worker instances:
```bash
# Scale to 3 worker instances
docker-compose up -d --scale app=3
```

Workers use database-level locking (`FOR UPDATE SKIP LOCKED`) to coordinate, so multiple instances can safely process jobs concurrently.

---

## 📊 Monitoring

### Key Metrics to Monitor

- **Job throughput**: Jobs completed per minute
- **Processing speed**: Rows processed per second
- **Error rate**: Failed rows / Total rows
- **Queue depth**: Number of PENDING jobs
- **Worker utilization**: Active workers / Total workers
- **S3 storage**: Total bytes stored

### Viewing Metrics
```bash
# Check job statistics
psql -h localhost -U postgres -d bulkops -c "
  SELECT 
    type,
    status,
    COUNT(*) as count,
    AVG(processed_rows) as avg_rows,
    SUM(failed_rows) as total_failed
  FROM jobs
  GROUP BY type, status
"

# Check recent jobs
psql -h localhost -U postgres -d bulkops -c "
  SELECT id, type, resource, status, processed_rows, created_at
  FROM jobs
  ORDER BY created_at DESC
  LIMIT 10
"
```

---
## 📄 License

This project is licensed under the MIT License.

---

## 🙏 Acknowledgments

- Built with [golang-gin-realworld-example-app](https://github.com/gothinkster/golang-gin-realworld-example-app) as the foundation
- Uses [LocalStack](https://www.localstack.cloud/) for local S3 emulation
- Inspired by production bulk data systems at scale
