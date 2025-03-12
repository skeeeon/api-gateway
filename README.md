# API Gateway with PocketBase Authentication

[![Go Report Card](https://goreportcard.com/badge/github.com/yourusername/api-gateway)](https://goreportcard.com/report/github.com/yourusername/api-gateway)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

A secure, high-performance API Gateway that provides authentication and authorization for HTTP APIs using PocketBase as the identity provider. The gateway maps MQTT-style topic pattern permissions to HTTP endpoints, creating a unified permission model that can be shared between HTTP and MQTT protocols.

![Architecture Diagram](docs/images/architecture.png)

## 🚀 Features

- **JWT Authentication**: Validates PocketBase JWT tokens
- **MQTT-Style Authorization**: Uses topic pattern matching for permissions
- **Wildcard Support**: Handles single-level (`+`) and multi-level (`#`) wildcards
- **Role-Based Access Control**: Configurable role-based permissions
- **Reverse Proxy**: Routes authenticated requests to backend services
- **Performance Optimized**: Efficient caching of user and role data
- **Metrics & Monitoring**: Prometheus metrics for observability
- **Dockerized**: Ready for containerized deployment
- **Scalable**: Stateless design for horizontal scaling

## 📖 Table of Contents

- [Installation](#-installation)
- [Quick Start](#-quick-start)
- [Architecture](#-architecture)
- [Configuration](#-configuration)
- [PocketBase Setup](#-pocketbase-setup)
- [Permission Model](#-permission-model)
- [API Reference](#-api-reference)
- [Metrics](#-metrics)
- [Development](#-development)
- [Security Considerations](#-security-considerations)
- [License](#-license)

## 📥 Installation

### Prerequisites

- Go 1.21 or later
- PocketBase instance
- Backend services to proxy to

### Using Go

```bash
# Clone the repository
git clone https://github.com/yourusername/api-gateway.git
cd api-gateway

# Build the application
go build -o api-gateway ./cmd/api-gateway

# Run the gateway
./api-gateway --config=configs/config.development.json
```

### Using Docker

```bash
# Pull the image
docker pull yourusername/api-gateway:latest

# Run the container
docker run -p 9000:9000 -v $(pwd)/configs/config.json:/app/config.json yourusername/api-gateway
```

### Using Docker Compose

```bash
# Start with Docker Compose
docker-compose up -d
```

## 🚀 Quick Start

1. **Set up PocketBase**:
   - Deploy PocketBase (see [PocketBase Setup](#-pocketbase-setup))
   - Create a service account for gateway authentication
   - Create roles with permission patterns

2. **Configure the gateway**:
   ```json
   {
     "server": {
       "host": "0.0.0.0",
       "port": 9000
     },
     "pocketbase": {
       "url": "http://localhost:8090",
       "serviceAccount": "admin@example.com",
       "servicePassword": "secure-password",
       "userCollection": "users",
       "roleCollection": "mqtt_roles"
     },
     "routes": [
       {
         "pathPrefix": "/api/v1/device-status",
         "targetUrl": "http://localhost:8080",
         "stripPrefix": false
       },
       {
         "pathPrefix": "/api/v1/sensor-data",
         "targetUrl": "http://localhost:8081",
         "stripPrefix": false
       }
     ],
     "logLevel": "info",
     "cacheTTLSeconds": 300
   }
   ```

3. **Start the gateway**:
   ```bash
   ./api-gateway --config=config.json
   ```

4. **Use the API with JWT Authentication**:
   ```bash
   # First authenticate with PocketBase to get a JWT token
   curl -X POST http://localhost:8090/api/collections/users/auth-with-password \
     -H "Content-Type: application/json" \
     -d '{"identity":"user@example.com","password":"password123"}'

   # Then use the token with your API requests
   curl -X GET http://localhost:9000/api/v1/device-status \
     -H "Authorization: Bearer YOUR_JWT_TOKEN"
   ```

## 🏗️ Architecture

The API Gateway operates as a reverse proxy with authentication and authorization layers:

1. **Client Request**: Client includes JWT token in the Authorization header
2. **Authentication**: Gateway validates the token with PocketBase
3. **Permission Check**: Gateway checks user role permissions against the request path
4. **Proxying**: If authorized, the request is forwarded to the appropriate backend service
5. **Response**: The backend service's response is returned to the client

### Components

- **Gateway Core**: Main HTTP server with middleware and proxy logic
- **PocketBase Client**: Communicates with PocketBase for user/role data
- **Permission Matcher**: Implements MQTT topic pattern matching
- **Cache**: Improves performance by caching authenticated users and roles
- **Metrics**: Exports Prometheus metrics for monitoring

## ⚙️ Configuration

The gateway is configured using a JSON configuration file:

| Section | Field | Description | Default |
|---------|-------|-------------|---------|
| server | host | Host to bind to | 0.0.0.0 |
| server | port | Port to listen on | 9000 |
| pocketbase | url | PocketBase instance URL | - |
| pocketbase | serviceAccount | PocketBase admin email | - |
| pocketbase | servicePassword | PocketBase admin password | - |
| pocketbase | userCollection | Name of users collection | users |
| pocketbase | roleCollection | Name of roles collection | mqtt_roles |
| routes | pathPrefix | HTTP path prefix to match | - |
| routes | targetUrl | Backend service URL | - |
| routes | stripPrefix | Whether to strip prefix before proxying | false |
| - | logLevel | Logging level (debug, info, warn, error) | info |
| - | cacheTTLSeconds | Cache TTL in seconds | 300 |

### Environment Variables

All configuration options can also be set using environment variables:

```bash
API_GATEWAY_SERVER_HOST=0.0.0.0
API_GATEWAY_SERVER_PORT=9000
API_GATEWAY_POCKETBASE_URL=http://pocketbase:8090
API_GATEWAY_POCKETBASE_SERVICEACCOUNT=admin@example.com
API_GATEWAY_POCKETBASE_SERVICEPASSWORD=secure-password
API_GATEWAY_LOGLEVEL=info
API_GATEWAY_CACHETTLSECONDS=300
```

Routes must be defined in the configuration file.

## 🗃️ PocketBase Setup

### 1. Install PocketBase

Download PocketBase from [pocketbase.io](https://pocketbase.io/), or use the Docker image:

```bash
docker run -p 8090:8090 -v pb_data:/pb_data ghcr.io/pocketbase/pocketbase:latest
```

### 2. Create Service Account

The gateway needs an admin account to access PocketBase:

1. Create an admin user in PocketBase
2. Use these credentials in the gateway configuration

### 3. Create Role Collection

Create a collection named `mqtt_roles` (or your preferred name) with fields:

| Field | Type | Description |
|-------|------|-------------|
| name | Text | Role name |
| publish_permissions | JSON | Array of publish permission patterns |
| subscribe_permissions | JSON | Array of subscribe permission patterns |

Example JSON values:
```json
// publish_permissions for an admin
["api/#"]

// subscribe_permissions for a reader
["api/v1/public/#", "api/+/device/+"]
```

### 4. Add Role Field to Users Collection

Add a relation field to the PocketBase users collection:

| Field | Type | Description |
|-------|------|-------------|
| role_id | Relation | Reference to a role in mqtt_roles |

### 5. Import Example Schema

You can import the example schema from `examples/pocketbase/pb_schema.json` to set up PocketBase quickly.

## 🔐 Permission Model

The permission model uses MQTT-style topic patterns to match HTTP paths:

### HTTP Method to Permission Type Mapping

- GET, HEAD, OPTIONS → subscribe permissions
- POST, PUT, PATCH, DELETE → publish permissions

### Path to Topic Conversion

HTTP paths are converted to topics by removing the leading slash:
- `/api/v1/device-status` → `api/v1/device-status`

### Wildcard Support

- `+` matches exactly one segment:
  - `api/+/device` matches `api/v1/device` but not `api/v1/device/123`
  
- `#` matches zero or more segments (must be at the end):
  - `api/v1/#` matches `api/v1/device`, `api/v1/device/123`, etc.

### Examples

| Permission Pattern | HTTP Method | HTTP Path | Result |
|-------------------|-------------|-----------|--------|
| `api/v1/device/+` | GET | `/api/v1/device/123` | ✅ Allowed |
| `api/v1/device/+` | GET | `/api/v1/device/123/status` | ❌ Denied |
| `api/v1/#` | GET | `/api/v1/device/123/status` | ✅ Allowed |
| `api/+/public/#` | GET | `/api/v2/public/data` | ✅ Allowed |
| `api/v1/device/+` | POST | `/api/v1/device/123` | ❌ Denied (wrong method) |

## 📚 API Reference

### Built-in Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check endpoint |
| `/metrics` | GET | Prometheus metrics |

### Headers Added to Proxied Requests

The gateway adds the following headers to requests forwarded to backend services:

| Header | Description |
|--------|-------------|
| X-User-ID | PocketBase user ID |
| X-Username | PocketBase username |
| X-Role-ID | PocketBase role ID |
| X-Role-Name | PocketBase role name |

## 📊 Metrics

The gateway exports Prometheus metrics at `/metrics`:

| Metric | Type | Description |
|--------|------|-------------|
| api_gateway_requests_total | Counter | Total number of HTTP requests processed |
| api_gateway_request_duration_seconds | Histogram | Duration of HTTP requests in seconds |
| api_gateway_auth_failures_total | Counter | Total number of authentication failures |
| api_gateway_cache_refreshes_total | Counter | Total number of cache refresh operations |
| api_gateway_cache_size | Gauge | Number of items in cache |
| api_gateway_active_connections | Gauge | Number of active connections |

## 🛠️ Development

### Project Structure

The project follows Go best practices with a clear separation between application-specific code and potentially reusable packages:

```
api-gateway/
├── cmd/api-gateway/         # Application entry point
├── internal/                # Application-specific packages (not importable by external code)
│   ├── cache/               # User and role caching
│   ├── config/              # Configuration loading
│   ├── gateway/             # API gateway implementation
│   ├── metrics/             # Prometheus metrics
│   └── pocketbase/          # PocketBase client
├── pkg/                     # Reusable packages (importable by external code)
│   └── permissions/         # MQTT topic pattern matching (potentially reusable)
├── configs/                 # Configuration files
├── docs/                    # Documentation
└── examples/                # Example configurations
```

The structure follows the modern Go convention:
- `cmd/`: Contains the main application entry points
- `internal/`: Houses application-specific code that shouldn't be imported by other projects
- `pkg/`: Contains potentially reusable packages that could be imported by other projects
- Supporting directories for configuration, documentation, and examples

### Building from Source

```bash
# Install dependencies
go mod tidy

# Run tests
go test ./...

# Build
go build -o api-gateway ./cmd/api-gateway
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...
```

## 🔒 Security Considerations

1. **Secure Service Account**: Use a dedicated admin account with a strong password
2. **HTTPS**: Deploy the gateway with TLS in production
3. **JWT Settings**: Configure secure JWT settings in PocketBase
4. **Network Isolation**: Run the gateway in a separate network from public internet
5. **Least Privilege**: Define minimal necessary permissions for each role
6. **Regular Audits**: Review permissions and access patterns regularly
7. **Rate Limiting**: Consider implementing rate limiting for API endpoints
8. **Monitoring**: Set up alerts for authentication failures and unusual traffic patterns

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
