# dns-api-go

The DNS API is a robust backend service designed to manage DNS records, IP addresses, and network resources efficiently. It acts as an intermediary between Yale's Spinup cloud platform and BlueCat's DNS/DHCP/IPAM system, providing a comprehensive set of operations for managing DNS infrastructure.

## Table of Contents

- [Features](#features)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [API Documentation](#api-documentation)
- [Authentication](#authentication)
- [API Endpoints](#api-endpoints)
- [BlueCat Integration](#bluecat-integration)
- [Error Handling](#error-handling)
- [Usage Examples](#usage-examples)
- [License](#license)

## Features

- **Comprehensive DNS Management**: Create, retrieve, and delete DNS records (Host, Alias, External Host)
- **IP Address Management**: Assign, retrieve, and delete IP addresses with CIDR support
- **Network Management**: List and retrieve network information
- **MAC Address Management**: Create, retrieve, and update MAC addresses
- **Entity Management**: Generic entity operations by ID with custom search capabilities
- **Zone Management**: Retrieve DNS zone information
- **BlueCat Integration**: Seamless integration with BlueCat Address Manager
- **Secure Authentication**: Token-based middleware for API security
- **Interactive Documentation**: Swagger/OpenAPI integration for API exploration
- **Account-based Routing**: Multi-tenant support with account-specific endpoints
- **Robust Error Handling**: Comprehensive error responses and logging
- **Health Monitoring**: Health check and metrics endpoints

## Prerequisites

- Go 1.21.x or later
- BlueCat Address Manager instance with API access
- Valid BlueCat account credentials
- Redis (optional, for advanced features)

## Installation

### Local Development

1. Clone the repository:
   ```bash
   git clone https://git.yale.edu/spinup/dns-api-go.git
   cd dns-api-go
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Create a configuration file based on the example:
   ```bash
   cp config/config.example.json config/config.json
   ```

4. Configure your settings in `config/config.json`:
   ```json
   {
     "listenAddress": ":8080",
     "org": "your-org",
     "token": "your-auth-token",
     "logLevel": "info",
     "bluecat": {
       "baseUrl": "https://your-bluecat-server",
       "username": "your-username",
       "password": "your-password",
       "account": "your-account",
       "viewId": "your-view-id"
     },
     "cidrFile": "path/to/cidr.json"
   }
   ```

5. Build and run the application:
   ```bash
   go build -o dns-api-go ./cmd/main.go
   ./dns-api-go
   ```

## Configuration

Configuration is managed via a JSON file (`config/config.json`). Key configuration options:

| Parameter | Type | Description |
|-----------|------|-------------|
| `listenAddress` | string | Server listen address (default: ":8080") |
| `org` | string | Organization identifier |
| `token` | string | Authentication token for API requests |
| `logLevel` | string | Logging level (debug, info, warn, error) |
| `bluecat.baseUrl` | string | BlueCat Address Manager base URL |
| `bluecat.username` | string | BlueCat username |
| `bluecat.password` | string | BlueCat password |
| `bluecat.account` | string | BlueCat account name |
| `bluecat.viewId` | string | BlueCat view ID |
| `cidrFile` | string | Path to CIDR configuration file |

## API Documentation

The DNS API includes interactive Swagger/OpenAPI documentation for easy exploration and testing.

**Access Swagger UI:**
- Start the API server
- Navigate to `http://localhost:8080/swagger/index.html`

The Swagger UI provides:
- Interactive endpoint testing
- Request/response examples
- Authentication information
- Model definitions

## Authentication

Authentication is accomplished via an encrypted pre-shared key passed via the `X-Auth-Token` header.

**Example:**
```bash
curl -H "X-Auth-Token: your-auth-token" \
     http://localhost:8080/v2/dns/your-account/zones
```

**Public Endpoints (no authentication required):**
- `GET /v2/dns/ping` - Health check
- `GET /v2/dns/version` - API version
- `GET /v2/dns/metrics` - Metrics
- `GET /swagger/*` - API documentation

## API Endpoints

All endpoints are prefixed with `/v2/dns` and most require an `{account}` parameter.

### Health & Info Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/ping` | Health check endpoint |
| `GET` | `/version` | API version information |
| `GET` | `/metrics` | Prometheus metrics |
| `GET` | `/` | Root endpoint with account info |
| `GET` | `/systeminfo` | BlueCat system information |

### Entity Management

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/{account}/search` | Custom entity search with filters |
| `GET` | `/{account}/id/{id}` | Get entity by ID |
| `DELETE` | `/{account}/id/{id}` | Delete entity by ID |

### Zone Management

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/{account}/zones` | Get DNS zones by hint |
| `GET` | `/{account}/zones/{id}` | Get specific zone by ID |

### DNS Record Management

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/{account}/records` | Get DNS records by type |
| `POST` | `/{account}/records` | Create DNS record |
| `GET` | `/{account}/records/{id}` | Get specific record by ID |
| `DELETE` | `/{account}/records/{id}` | Delete record by ID |

**Supported Record Types:**
- `HostRecord` - A records
- `AliasRecord` - CNAME records
- `ExternalHostRecord` - External host records

### Network Management

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/{account}/networks` | Get networks by hint |
| `GET` | `/{account}/networks/{id}` | Get specific network by ID |

### IP Address Management

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/{account}/ips/cidrs` | Get CIDR configuration |
| `POST` | `/{account}/ips` | Assign next available IP |
| `GET` | `/{account}/ips/{ip}` | Get IP address details |
| `DELETE` | `/{account}/ips/{ip}` | Delete IP address |

### MAC Address Management

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/{account}/macs` | Create MAC address |
| `GET` | `/{account}/macs/{mac}` | Get MAC address details |
| `PUT` | `/{account}/macs/{mac}` | Update MAC address |

## BlueCat Integration

The DNS API integrates with BlueCat Address Manager using the following mapping:

| DNS API Endpoint | BlueCat API Route | Purpose |
|------------------|-------------------|---------|
| `GET /zones` | `/v1/getZonesByHint` | Retrieve DNS zones |
| `GET /records` | `/v1/getHostRecordsByHint`, `/v1/getAliasesByHint` | Get DNS records |
| `POST /records` | `/v1/addHostRecord`, `/v1/addAliasRecord` | Create DNS records |
| `GET /networks` | `/v1/getIP4NetworksByHint` | Get IPv4 networks |
| `POST /ips` | `/v1/assignNextAvailableIP4Address` | Assign IP addresses |
| `GET /ips/{ip}` | `/v1/getIP4Address` | Get IP details |
| `POST /macs` | `/v1/addMACAddress` | Create MAC addresses |
| `GET /macs/{mac}` | `/v1/getMACAddress` | Get MAC details |
| `GET /search` | `/v1/customSearch` | Custom entity search |

## Error Handling

The API returns structured error responses with appropriate HTTP status codes:

```json
{
  "error": "Error description",
  "code": 400,
  "details": "Additional error details"
}
```

**Common HTTP Status Codes:**
- `200` - Success
- `201` - Created
- `204` - No Content
- `400` - Bad Request
- `401` - Unauthorized
- `403` - Forbidden
- `404` - Not Found
- `409` - Conflict
- `500` - Internal Server Error

## Usage Examples

### Create a DNS Host Record

```bash
curl -X POST \
  -H "X-Auth-Token: your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "HostRecord",
    "record": "server-001.example.com",
    "target": "10.0.1.15",
    "ttl": 300,
    "properties": "department=IT|environment=prod"
  }' \
  http://localhost:8080/v2/dns/your-account/records
```

### Assign Next Available IP

```bash
curl -X POST \
  -H "X-Auth-Token: your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "network_id": 12345,
    "mac": "00:11:22:33:44:55",
    "hostname": "server-001",
    "reverse": true
  }' \
  http://localhost:8080/v2/dns/your-account/ips
```

### Search for Entities

```bash
curl -G \
  -H "X-Auth-Token: your-token" \
  -d "type=HostRecord" \
  -d "filters=name=server*|address=10.0.1.*" \
  http://localhost:8080/v2/dns/your-account/search
```


## License

GNU Affero General Public License v3.0 (GNU AGPLv3)
Copyright © 2023 Yale University
