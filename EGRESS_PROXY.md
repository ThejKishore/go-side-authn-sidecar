# Egress Proxy Sidecar

The egress proxy is a sidecar service that handles outbound HTTP requests from the main application container. It provides OAuth token management and URL rewriting capabilities.

## Overview

The egress proxy runs on port `3002` and acts as an intermediary for all outbound requests from the main container. It supports multiple OAuth providers and can automatically attach bearer tokens to requests based on the configured IDP type.

## Features

- **URL Rewriting**: Replace the base URL using the `X-Backend-Url` header
- **OAuth Token Management**: Automatically fetch and refresh tokens from configured OAuth providers
- **Multiple IDP Support**: Support for Ping, Okta, Keycloak, and custom providers
- **Header Forwarding**: Forward all incoming headers to the backend (except proxy-specific headers)
- **Error Passthrough**: Backend errors are forwarded as-is to the client
- **No-IDP Mode**: Support for calls without authentication (noIdp mode)

## Configuration

The egress proxy is configured via `egress-config.yaml` at the project root. Here's an example configuration:

```yaml
multi-oauth-client-config:
  "ping":
    tokenUrl: https://ping.example.com/authorization/token
    clientId: your-client-id
    clientSecret: your-client-secret
    clientCertificate: /path/to/cert.pem
    scope:
      - openid

  "okta":
    tokenUrl: https://okta.example.com/oauth2/v1/token
    clientId: your-client-id
    clientSecret: your-client-secret
    clientCertificate: ""
    scope:
      - openid

  "keycloak":
    tokenUrl: http://localhost:8080/realms/baeldung-keycloak/protocol/openid-connect/token
    clientId: your-client-id
    clientSecret: your-client-secret
    clientCertificate: ""
    scope:
      - openid
```

### Configuration Fields

- `tokenUrl`: The OAuth token endpoint URL
- `clientId`: OAuth client ID
- `clientSecret`: OAuth client secret
- `clientCertificate`: (Optional) Path to client certificate for mTLS (PEM format)
- `scope`: Array of OAuth scopes to request

## Usage

### Making Requests

To make a request through the egress proxy:

1. **Required Headers**:
   - `X-Backend-Url`: The backend service URL
   - `X-Idp-Type`: The IDP type (ping, okta, keycloak, or noIdp)

2. **Example Request**:

```bash
curl -X GET http://localhost:3002/api/endpoint \
  -H "X-Backend-Url: https://api.example.com" \
  -H "X-Idp-Type: okta" \
  -H "X-Custom-Header: custom-value"
```

This request will:
- Replace the base URL with `https://api.example.com`
- Attach the Okta bearer token if available
- Forward `X-Custom-Header` to the backend
- Forward the response back to the client

### POST/PUT/PATCH Requests

For requests with a body, the proxy will automatically forward the request body to the backend:

```bash
curl -X POST http://localhost:3002/api/resource \
  -H "X-Backend-Url: https://api.example.com" \
  -H "X-Idp-Type: keycloak" \
  -H "Content-Type: application/json" \
  -d '{"key": "value"}'
```

### No Authentication Mode

For requests that don't require authentication:

```bash
curl -X GET http://localhost:3002/public/endpoint \
  -H "X-Backend-Url: https://public-api.example.com" \
  -H "X-Idp-Type: noIdp"
```

## Token Management

### Token Refresh

Tokens are automatically refreshed every 10 minutes. The token manager:

1. Fetches tokens for all configured IDP types on startup
2. Stores tokens in `/tmp/egress-tokens/` directory (e.g., `ping-token.txt`, `okta-token.txt`)
3. Maintains tokens in memory with expiration tracking
4. Automatically refreshes tokens before they expire

### Token Storage

Tokens are stored in two places:

1. **Memory**: For faster access with expiration tracking
2. **File System**: In `/tmp/egress-tokens/{idp-type}-token.txt` for persistence

### Token Lifecycle

- Tokens are fetched immediately on application startup
- Tokens are refreshed every 10 minutes (configurable)
- Tokens are stored with their expiration time
- Expired tokens in memory are replaced with fresh ones from the file system or re-fetched

## Header Handling

### Headers Forwarded to Backend

All incoming headers are forwarded to the backend except:
- `Host` (set by the HTTP client)
- `Content-Length` (set by the HTTP client)
- `X-Backend-Url` (proxy-specific)
- `X-Idp-Type` (proxy-specific)

### Headers Added by Proxy

The proxy automatically adds:
- `Authorization: Bearer {token}` (if IDP type is not noIdp and token is available)

## Error Handling

The proxy follows these error handling principles:

1. **Backend Errors**: All errors from the backend (4xx, 5xx) are forwarded as-is to the client
2. **Missing Headers**: Returns `400 Bad Request` if `X-Backend-Url` is missing
3. **Configuration Errors**: Logged but don't prevent proxy operation in noIdp mode
4. **Token Fetch Errors**: Logged but requests continue without authentication

## Architecture

### Components

1. **egressconfig**: Manages configuration loading and access
2. **oauthclient**: Handles OAuth token fetching from providers
3. **tokenstorage**: Manages token storage (memory and file system)
4. **tokenmanager**: Orchestrates token fetching and refreshing
5. **egressproxy**: Main HTTP handler for the proxy

### Token Refresh Flow

```
Application Start
    ↓
Load Configuration (egressconfig)
    ↓
Start Token Manager (tokenmanager)
    ↓
For Each IDP Type:
    ├─ Create OAuth Client (oauthclient)
    ├─ Fetch Token
    └─ Store Token (tokenstorage)
    ↓
Periodic Refresh (every 10 minutes)
    ├─ For Each IDP Type:
    │   ├─ Create OAuth Client
    │   ├─ Fetch New Token
    │   └─ Update Storage
```

### Request Flow

```
Incoming Request
    ↓
Extract X-Backend-Url and X-Idp-Type Headers
    ↓
Retrieve Token (if IDP type != noIdp)
    ↓
Create HTTP Request
    ├─ Set Target URL
    ├─ Forward Headers
    ├─ Add Authorization Header (if applicable)
    └─ Forward Body (if present)
    ↓
Execute Backend Request
    ↓
Forward Response
    ├─ Status Code
    ├─ Headers
    └─ Body
```

## Troubleshooting

### Token Not Available

If tokens are not available when needed:

1. Check configuration file exists and is properly formatted
2. Verify IDP credentials (clientId, clientSecret)
3. Check logs for token fetch errors
4. Verify network connectivity to token endpoint

### Requests Failing with 502 Bad Gateway

This usually indicates:

1. Backend service is unreachable
2. Backend URL is invalid
3. Network issues

### Headers Not Forwarded

Ensure you're not using the reserved header names:
- `X-Backend-Url`
- `X-Idp-Type`

These are consumed by the proxy and not forwarded to the backend.

## Running the Proxy

The egress proxy is started automatically when the main application starts:

```go
func main() {
	go egressProxy()
	// ... rest of main
}
```

It listens on `http://localhost:3002` by default.

## Testing

Run the test suite:

```bash
go test ./internal/egressconfig/...
go test ./internal/oauthclient/...
go test ./internal/tokenstorage/...
go test ./internal/tokenmanager/...
go test ./internal/egressproxy/...
```

Or run all tests:

```bash
go test ./...
```

