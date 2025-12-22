# Egress Proxy Quick Start Guide

## 1. Setup

### Prerequisites
- Go 1.25+
- Fiber v3 (already included in `go.mod`)

### Build the Application
```bash
cd /Users/thejkaruneegar/GolandProjects/reverseProxy
go build ./cmd/reverse-proxy
```

## 2. Configure OAuth Providers

Edit `egress-config.yaml` with your OAuth credentials:

```yaml
multi-oauth-client-config:
  "keycloak":
    tokenUrl: http://localhost:8080/realms/your-realm/protocol/openid-connect/token
    clientId: your-client-id
    clientSecret: your-client-secret
    clientCertificate: ""
    scope:
      - openid

  "okta":
    tokenUrl: https://your-domain.okta.com/oauth2/v1/token
    clientId: your-client-id
    clientSecret: your-client-secret
    clientCertificate: ""
    scope:
      - openid
```

## 3. Run the Application

```bash
./reverse-proxy
```

Output:
```
2025/12/17 20:50:00 Token refresh started for all configured IDP types
[3001] GET /api/users (12.5ms)
[3002] GET /api/endpoint (8.3ms)
```

Two services will start:
- **Reverse Proxy**: `http://localhost:3001`
- **Egress Proxy**: `http://localhost:3002`

## 4. Test the Egress Proxy

### Test 1: Simple GET Request with Okta Authentication
```bash
curl -v http://localhost:3002/api/users \
  -H "X-Backend-Url: https://api.example.com" \
  -H "X-Idp-Type: okta"
```

Expected Flow:
1. Proxy retrieves Okta token
2. Adds `Authorization: Bearer {token}` header
3. Forwards request to `https://api.example.com/api/users`
4. Returns backend response

### Test 2: POST Request with Body
```bash
curl -X POST http://localhost:3002/api/users \
  -H "X-Backend-Url: https://api.example.com" \
  -H "X-Idp-Type: keycloak" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "John Doe",
    "email": "john@example.com"
  }'
```

### Test 3: Request with Custom Headers
```bash
curl -v http://localhost:3002/api/users \
  -H "X-Backend-Url: https://api.example.com" \
  -H "X-Idp-Type: okta" \
  -H "X-Request-ID: 12345" \
  -H "Accept: application/json"
```

The `X-Request-ID` and `Accept` headers will be forwarded to the backend.

### Test 4: No Authentication Mode
```bash
curl http://localhost:3002/public/data \
  -H "X-Backend-Url: https://public-api.example.com" \
  -H "X-Idp-Type: noIdp"
```

### Test 5: Missing X-Backend-Url (Should Fail)
```bash
curl http://localhost:3002/api/users \
  -H "X-Idp-Type: okta"
```

Expected Response:
```
Status: 400
Body: X-Backend-Url header is required
```

## 5. Monitor Token Refresh

Check token files:
```bash
ls -la /tmp/egress-tokens/
```

Output:
```
-rw------- okta-token.txt
-rw------- keycloak-token.txt
```

View token content (for debugging):
```bash
cat /tmp/egress-tokens/okta-token.txt
```

## 6. Run Tests

```bash
# All egress proxy tests
go test ./internal/egressconfig \
        ./internal/tokenstorage \
        ./internal/tokenmanager \
        ./internal/egressproxy -v

# Expected Output
# === RUN TestLoadConfig
# --- PASS: TestLoadConfig (0.00s)
# === RUN TestSaveAndGetToken
# --- PASS: TestSaveAndGetToken (0.00s)
# ...
# PASS
```

## 7. Common Issues & Solutions

### Issue: "X-Backend-Url header is required"
**Solution**: Always include `X-Backend-Url` header in requests

```bash
curl http://localhost:3002/api/endpoint \
  -H "X-Backend-Url: https://api.example.com" \
  -H "X-Idp-Type: okta"
```

### Issue: "backend request failed: dial tcp: lookup ... no such host"
**Solution**: Verify backend URL is correct and network is accessible

```bash
# Test backend connectivity
curl https://api.example.com/api/endpoint
```

### Issue: Tokens not refreshing
**Solution**: Check `egress-config.yaml` credentials and token endpoint

```bash
# Manually test token endpoint
curl -X POST https://okta.example.com/oauth2/v1/token \
  -d "grant_type=client_credentials&client_id=YOUR_ID&client_secret=YOUR_SECRET&scope=openid"
```

### Issue: "PKCS12 certificates not directly supported"
**Solution**: Convert PKCS12 to PEM format

```bash
openssl pkcs12 -in certificate.pfx -out certificate.pem -nodes
```

## 8. Integration with Your Application

From your main container, make egress calls to the sidecar:

**Python Example:**
```python
import requests

response = requests.get(
    "http://localhost:3002/api/users",
    headers={
        "X-Backend-Url": "https://api.example.com",
        "X-Idp-Type": "okta"
    }
)
```

**Node.js Example:**
```javascript
const fetch = require('node-fetch');

const response = await fetch('http://localhost:3002/api/users', {
  headers: {
    'X-Backend-Url': 'https://api.example.com',
    'X-Idp-Type': 'okta'
  }
});
```

**Go Example:**
```go
req, _ := http.NewRequest("GET", "http://localhost:3002/api/users", nil)
req.Header.Set("X-Backend-Url", "https://api.example.com")
req.Header.Set("X-Idp-Type", "okta")

client := &http.Client{}
resp, _ := client.Do(req)
```

## 9. Docker Deployment

### Dockerfile Example

```dockerfile
FROM golang:1.25 as builder
WORKDIR /app
COPY .. .
RUN go build -o reverse-proxy ./cmd/reverse-proxy

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/reverse-proxy /app/
COPY ../egress-config.yaml /app/
WORKDIR /app
EXPOSE 3001 3002
CMD ["./reverse-proxy"]
```

### Docker Compose Example
```yaml
version: '3.8'
services:
  app:
    image: my-app:latest
    ports:
      - "8080:8080"
    depends_on:
      - egress-proxy
  
  egress-proxy:
    build: ./reverseProxy
    ports:
      - "3001:3001"
      - "3002:3002"
    environment:
      - OKTA_CLIENT_ID=${OKTA_CLIENT_ID}
      - OKTA_CLIENT_SECRET=${OKTA_CLIENT_SECRET}
    volumes:
      - ./egress-config.yaml:/app/egress-config.yaml
```

## 10. Performance Tuning

### Token Refresh Interval
Edit `cmd/reverse-proxy/main.go`:
```go
// Change from 10 * time.Minute to desired interval
tokenMgr.StartTokenRefresh(5 * time.Minute)
```

### HTTP Client Timeout
Edit `internal/oauthclient/client.go`:
```go
httpClient := &http.Client{
    Timeout: 30 * time.Second, // Increase if needed
}
```

## 11. Monitoring & Logging

### Check Logs
```bash
./reverse-proxy | grep -i "token\|error\|failed"
```

### Example Log Output
```
2025/12/17 20:50:00 Token refresh started for all configured IDP types
2025/12/17 20:50:01 Successfully refreshed token for IDP type 'okta'
2025/12/17 20:50:01 Successfully refreshed token for IDP type 'keycloak'
2025/12/17 20:50:02 Backend request failed: dial tcp: connection refused
```

## 12. Next Steps

1. ✅ Build and run the application
2. ✅ Configure OAuth providers in `egress-config.yaml`
3. ✅ Test with sample requests
4. ✅ Integrate with your application
5. ✅ Monitor token refresh and requests
6. ✅ Deploy to production

## Documentation References

- **[EGRESS_PROXY.md](./EGRESS_PROXY.md)** - Comprehensive documentation
- **[IMPLEMENTATION_SUMMARY.md](./IMPLEMENTATION_SUMMARY.md)** - Implementation details
- **[Fiber Documentation](https://docs.gofiber.io/api/fiber)** - Framework reference

## Support

For issues or questions:
1. Check logs for error messages
2. Verify configuration syntax
3. Test OAuth endpoints manually
4. Review documentation
5. Run test suite to validate setup

