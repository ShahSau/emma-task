# API Reference

Complete API documentation for the Bulk Import/Export system.

---

## Table of Contents

1. [Import Endpoints](#import-endpoints)
2. [Export Endpoints](#export-endpoints)
3. [Job Status Endpoints](#job-status-endpoints)
4. [Data Formats](#data-formats)
5. [Error Handling](#error-handling)
6. [Rate Limits](#rate-limits)

---

## Import Endpoints

### Create Import Job

Upload a file and create an async import job.

**Endpoint:** `POST /v1/imports`

**Headers:**
```
Content-Type: multipart/form-data
Idempotency-Key: <optional-unique-key>
```

**Form Data:**
- `file`: The file to import (required)
- `resource`: Resource type - `users`, `articles`, or `comments` (required)

**Supported File Formats:**
- **CSV**: For users only (`.csv`)
- **NDJSON**: For all resources (`.ndjson`)
- **JSON Array**: For all resources (`.json`)

**Example: Upload CSV File**
```bash
curl -X POST http://localhost:8080/v1/imports \
  -F "file=@users.csv" \
  -F "resource=users"
```

**Example: With Idempotency Key**
```bash
curl -X POST http://localhost:8080/v1/imports \
  -H "Idempotency-Key: unique-key-123" \
  -F "file=@users.csv" \
  -F "resource=users"
```

**Response:** `202 Accepted`
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "PENDING",
  "created_at": "2026-02-05T12:00:00Z"
}
```

**Error Response:** `400 Bad Request`
```json
{
  "error": "resource is required"
}
```

---

## Export Endpoints

### Synchronous Streaming Export

Stream data directly to response (for small datasets < 100k records).

**Endpoint:** `GET /v1/exports`

**Query Parameters:**
- `resource`: Resource type - `users`, `articles`, or `comments` (required)
- `format`: Output format - `ndjson`, `csv`, or `json` (default: `ndjson`)
- `author`: Filter articles by author username (optional)
- `username`: Filter users by username (optional)
- `tag`: Filter articles by tag (optional)
- `article`: Filter comments by article slug (optional)

**Example: Export All Users as NDJSON**
```bash
curl "http://localhost:8080/v1/exports?resource=users&format=ndjson" \
  > users.ndjson
```

**Example: Export Articles by Author as CSV**
```bash
curl "http://localhost:8080/v1/exports?resource=articles&format=csv&author=johndoe" \
  > johndoe_articles.csv
```

**Example: Export as JSON Array**
```bash
curl "http://localhost:8080/v1/exports?resource=comments&format=json&article=hello-world" \
  > comments.json
```

**Response:** `200 OK`
```
Content-Type: application/x-ndjson
Content-Disposition: attachment; filename=users.ndjson
Transfer-Encoding: chunked

{"id":"user-1","email":"john@example.com","username":"john",...}
{"id":"user-2","email":"jane@example.com","username":"jane",...}
```

**Error Response:** `400 Bad Request`
```json
{
  "error": "Invalid resource"
}
```

---

### Create Async Export Job

Create a background export job (for large datasets > 100k records).

**Endpoint:** `POST /v1/exports`

**Headers:**
```
Content-Type: application/json
```

**Request Body:**
```json
{
  "resource": "articles",
  "format": "ndjson",
  "filters": {
    "author": "johndoe",
    "status": "published"
  }
}
```

**Parameters:**
- `resource`: Resource type (required)
- `format`: Output format - `ndjson`, `csv`, or `json` (default: `ndjson`)
- `filters`: Filter criteria (optional)

**Example: Export All Users**
```bash
curl -X POST http://localhost:8080/v1/exports \
  -H "Content-Type: application/json" \
  -d '{
    "resource": "users",
    "format": "ndjson"
  }'
```

**Example: Export Articles with Filters**
```bash
curl -X POST http://localhost:8080/v1/exports \
  -H "Content-Type: application/json" \
  -d '{
    "resource": "articles",
    "format": "csv",
    "filters": {
      "author": "johndoe"
    }
  }'
```

**Response:** `202 Accepted`
```json
{
  "job_id": "660e8400-e29b-41d4-a716-446655440000",
  "status": "PENDING",
  "message": "Export job started"
}
```

**Error Response:** `400 Bad Request`
```json
{
  "error": "Invalid resource"
}
```

---

## Job Status Endpoints

### Get Job Status & Downloads

Check the status of **ANY** job (Import or Export).
If the job is `COMPLETED` (or has partial errors), a `download_url` is provided to retrieve the result file (Exported Data or Import Error Report).

**Endpoint:** `GET /v1/jobs/:id`

**Parameters:**
- `id`: Job UUID (required)

**Example:**
```bash
curl http://localhost:8080/v1/jobs/660e8400-e29b-41d4-a716-446655440000
```

**Response (Processing):** `200 OK`
```json
{
  "job_id": "660e8400-e29b-41d4-a716-446655440000",
  "type": "EXPORT",
  "resource": "articles",
  "status": "PROCESSING",
  "processed_rows": 25000,
  "total_rows": 50000,
  "created_at": "2026-02-05T13:00:00Z",
  "updated_at": "2026-02-05T13:00:30Z"
}
```

**Response (Export Completed):** `200 OK`
```json
{
  "job_id": "660e8400-e29b-41d4-a716-446655440000",
  "type": "EXPORT",
  "resource": "articles",
  "status": "COMPLETED",
  "processed_rows": 50000,
  "total_rows": 50000,
  "download_url": "https://s3.../exports/articles/articles-660e8400.ndjson?signature=..."
}
```

**Response (Import Completed with Errors):** `200 OK`
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "IMPORT",
  "resource": "users",
  "status": "COMPLETED",
  "processed_rows": 10000,
  "failed_rows": 5,
  "download_url": "https://s3.../imports/errors/users-550e8400.csv?signature=..."
}
```

**Error Response:** `404 Not Found`
```json
{
  "error": "Job not found"
}
```

---

## Data Formats

### Users (CSV)

**Headers:**
```csv
id,email,name,role,active,created_at,updated_at
```

### Users (NDJSON)

**Example:**
```json
{"id":"user-1","email":"john@example.com","username":"john","bio":"Developer","image":null}
{"id":"user-2","email":"jane@example.com","username":"jane","bio":"Designer","image":null}
```

---

## Error Handling

### Validation Errors

**Import validation errors are recorded but processing continues:**

**Error Types:**
- `VALIDATION_ERROR`: Invalid field value
- `DEPENDENCY_ERROR`: Referenced entity doesn't exist
- `PARSE_ERROR`: Invalid JSON format
- `INSERT_ERROR`: Database constraint violation

---

## Rate Limits

**Import Jobs:**
- Max concurrent jobs: Unlimited (handled by worker pool)
- Max file size: 5GB (configurable)
- Max records per job: 1,000,000+ (no hard limit)

**Export Jobs:**
- Max concurrent exports: Unlimited
- Sync export limit: 100,000 records (use async for larger)

**Recommendations:**
- Use async exports for datasets > 10,000 records
- Use sync exports for quick downloads < 10,000 records
- Use idempotency keys to prevent duplicate imports