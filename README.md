# API Gateway with PocketBase Authentication

A secure, high-performance API Gateway that provides authentication and authorization for HTTP APIs using PocketBase as the identity provider. The gateway uses MQTT/NATS-style topic pattern matching for permissions, creating a unified permission model across protocols.

## Features

- 🔐 JWT Authentication with PocketBase integration
- 🔑 MQTT/NATS-style permission pattern matching
- 🚦 Reverse proxy with configurable routing
- 🧠 Intelligent caching for optimal performance
- 📊 Prometheus metrics for comprehensive monitoring
- 📝 Enhanced logging with multiple output options
- 🔄 Graceful shutdown and connection handling
- 🔍 Detailed permission debugging
- 🔧 Comprehensive configuration system
- 🚀 Stateless design for horizontal scaling

## Quick Start

1. Clone the repository:
```bash
git clone https://github.com/skeeeon/api-gateway
cd api-gateway
```

2. Copy the example configuration:
```bash
cp configs/config.example.json configs/config.json
```

3. Build the binary:
```bash
go build -o api-gateway ./cmd/api-gateway
```

4. Start the gateway:
```bash
./api-gateway --config=configs/config.json
```

## Project Structure

```
api-gateway/
├── cmd/
│   └── api-gateway/
│       └── main.go                   # Application entry point
├── configs/
│   └── config.json                   # Configuration file
├── internal/
│   ├── cache/
│   │   └── cache.go                  # In-memory caching for users and roles
│   ├── config/
│   │   └── config.go                 # Configuration structures and loading
│   ├── gateway/
│   │   └── gateway.go                # Core API gateway implementation
│   ├── logger/
│   │   └── logger.go                 # Enhanced logging with multiple outputs
│   ├── metrics/
│   │   └── metrics.go                # Prometheus metrics definitions
│   └── pocketbase/
│       └── client.go                 # PocketBase API client with connection pooling
├── pkg/
│   └── permissions/
│       ├── matcher.go                # Permission pattern matching
│       └── matcher_test.go           # Tests for pattern matching
├── docs/
│   └── permissions.md                # Permission system documentation
├── go.mod
└── README.md
```

## Prerequisites

- Go 1.21 or higher
- PocketBase instance (for user authentication and role management)
- Backend services to proxy to

## Configuration

The application uses a JSON configuration file with optional environment variable overrides.

### Configuration File Structure

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
      "stripPrefix": false,
      "protected": false
    },
    {
      "pathPrefix": "/api/v1/sensor-data",
      "targetUrl": "http://localhost:8081",
      "stripPrefix": false,
      "protected": true
    },
    {
      "pathPrefix": "/api/v2",
      "targetUrl": "http://localhost:8082",
      "stripPrefix": true,
      "protected": true
    }
  ],
  "logging": {
    "level": "info",
    "outputs": ["console", "file"],
    "filePath": "/var/log/api-gateway/api-gateway.log",
    "maxSizeMB": 100,
    "maxAgeDays": 30,
    "maxBackups": 5,
    "compress": true
  },
  "cacheTTLSeconds": 300
}
```

### Configuration Sections

#### Server Settings
- `host`: Host to bind to (default: "0.0.0.0")
- `port`: Port to listen on (default: 9000)

#### PocketBase Settings
- `url`: PocketBase instance URL (required)
- `serviceAccount`: Admin email for service authentication (required)
- `servicePassword`: Admin password for service authentication (required)
- `userCollection`: Name of users collection (default: "users")
- `roleCollection`: Name of roles collection (default: "mqtt_roles")

#### Routes Configuration
Array of proxy routes, each with:
- `pathPrefix`: HTTP path prefix to match (required)
- `targetUrl`: Backend service URL (required)
- `stripPrefix`: Whether to strip prefix before proxying (default: false)
- `protected`: Whether the route requires authentication (default: true)

#### Logging Configuration
- `level`: Log level (debug, info, warn, error) (default: "info")
- `outputs`: Log output destinations (["console"], ["file"], or ["console", "file"])
- `filePath`: Log file path (required when file output is enabled)
- `maxSizeMB`: Maximum log file size before rotation in MB (default: 100)
- `maxAgeDays`: Maximum days to retain old log files (default: 30)
- `maxBackups`: Maximum number of old log files to retain (default: 5)
- `compress`: Whether to compress rotated log files (default: true)

#### Cache Settings
- `cacheTTLSeconds`: Cache TTL in seconds (default: 300)

### Environment Variables

All configuration options can be set using environment variables with the `API_GATEWAY_` prefix:

```bash
API_GATEWAY_SERVER_HOST=0.0.0.0
API_GATEWAY_SERVER_PORT=9000
API_GATEWAY_POCKETBASE_URL=http://pocketbase:8090
API_GATEWAY_POCKETBASE_SERVICEACCOUNT=admin@example.com
API_GATEWAY_POCKETBASE_SERVICEPASSWORD=secure-password
API_GATEWAY_LOGGING_LEVEL=info
API_GATEWAY_LOGGING_OUTPUTS=console,file
API_GATEWAY_LOGGING_FILEPATH=/var/log/api-gateway.log
API_GATEWAY_CACHETTLSECONDS=300
```

### Command Line Flags

```bash
Usage of api-gateway:
  --config string
        path to config file (default "config.json")
```

### Protected vs Unprotected Routes

The API Gateway supports both authenticated and unauthenticated routes:

- **Protected Routes**: Require a valid JWT token and permission check (default)
- **Unprotected Routes**: Allow public access without authentication

This enables common authentication workflows where the gateway sits in front of your authentication service:

```json
{
  "routes": [
    {
      "pathPrefix": "/auth",
      "targetUrl": "http://pocketbase:8090",
      "stripPrefix": false,
      "protected": false  // Public authentication endpoints
    },
    {
      "pathPrefix": "/api",
      "targetUrl": "http://api-service:8000",
      "stripPrefix": false,
      "protected": true   // Protected API endpoints
    }
  ]
}
```

With this configuration:
1. Users can access `/auth/users/auth-with-password` to authenticate with PocketBase
2. PocketBase returns a JWT token
3. Users include this token in requests to `/api/...` endpoints
4. API Gateway validates the token and checks permissions before proxying to the API service

## Permission System

The gateway uses an MQTT/NATS-style topic pattern matching system for permissions.

### Permission Types

- **Publish Permissions**: Control write operations (POST, PUT, PATCH, DELETE)
- **Subscribe Permissions**: Control read operations (GET, HEAD, OPTIONS)

### Wildcard System

#### MQTT Wildcards

- `+` matches exactly one segment:
  - `api/+/devices` matches `api/v1/devices` but not `api/v1/devices/123`
  
- `#` matches zero or more segments:
  - `api/v1/#` matches `api/v1`, `api/v1/devices`, `api/v1/devices/123`, etc.

#### NATS Wildcards

- `*` matches exactly one segment:
  - `api.*.devices` matches `api.v1.devices` but not `api.v1.devices.123`
  
- `>` matches one or more segments:
  - `api.v1.>` matches `api.v1.devices`, `api.v1.devices.123`, etc.

### Example Permission Patterns

```json
{
  "publish_permissions": [
    "api/v1/devices/+/update",  // MQTT format
    "api.v2.devices.*.config"   // NATS format
  ],
  "subscribe_permissions": [
    "api/v1/#",                 // MQTT format
    "api.v2.public.>"           // NATS format
  ]
}
```

## Metrics

The gateway exposes Prometheus metrics at `/metrics` for monitoring:

### Available Metrics

1. **Request Metrics**:
   - `api_gateway_requests_total` (counter) - Total number of HTTP requests processed
   - `api_gateway_request_duration_seconds` (histogram) - Duration of HTTP requests

2. **Authentication Metrics**:
   - `api_gateway_auth_failures_total` (counter) - Authentication failures by reason

3. **Cache Metrics**:
   - `api_gateway_cache_refreshes_total` (counter) - Cache refresh operations
   - `api_gateway_cache_size` (gauge) - Size of cache by type (users, roles)

4. **Connection Metrics**:
   - `api_gateway_active_connections` (gauge) - Number of active connections

### Prometheus Configuration

Example Prometheus configuration:
```yaml
scrape_configs:
  - job_name: 'api-gateway'
    static_configs:
      - targets: ['localhost:9000']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

## Performance Optimization

The gateway is optimized for performance through several mechanisms:

### HTTP Connection Pooling
- Optimized connection pooling for PocketBase communication
- Persistent connections with configurable limits (100 max idle connections)
- Reduced latency by eliminating connection establishment overhead
- Connection keepalive for improved throughput
- Configurable timeout settings to prevent connection leaks
- Support for HTTP/2 when available

### Caching
- In-memory caching of user and role data
- Configurable TTL for cache entries
- Automatic cache refreshing

### Efficient Permission Matching
- Fast topic pattern matching algorithm
- Indexed lookup for quick permission checking
- Support for both MQTT and NATS style wildcards

### Connection Management
- Proper connection handling
- Graceful shutdown with timeout

### Performance Expectations
- **Throughput**: 500-5,000 requests/second depending on cache hit ratio
- **Latency**: 5-20ms for cached requests, 50-200ms for cache misses
- **Scalability**: Horizontal scaling with stateless design
- **Connection efficiency**: Significantly reduced resource usage under load

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
