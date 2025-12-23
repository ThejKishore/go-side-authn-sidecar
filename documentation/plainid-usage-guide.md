# PlainId Authorization Usage Guide

## Overview

This guide explains how to integrate plainId fine-grained authorization into your middleware or authorization layer. The plainId authorization class provides dynamic policy-based access control for your API endpoints.

## Quick Start

### 1. Configure authorization.yaml

First, create or update your `authorization.yaml` file with plainId configuration:

```yaml
finegrain-check:
  enabled: true
  validation-url: "http://plainid-service:8080/fga/api/runtime/5.0/decisions/permit-deny"
  client-id: "your-client-id"
  client-secret: "your-client-secret"
  client-auth-method: "client_secret_basic"
  resource-map:
    "[/api/transactions:POST]":
      roles: ["ROLE_USER"]
      ruleset-name: "transactions"
      ruleset-id: "1001"
      body:
        amount: $.amount
        currency: $.currency
        recipientId: $.recipientId
```

### 2. Load Configuration in Your Application

```go
import "reverseProxy/internal/authorization"

func init() {
    if err := authorization.Load("authorization.yaml"); err != nil {
        log.Fatalf("Failed to load authorization config: %v", err)
    }
}
```

### 3. Use in Middleware

Here's an example using Fiber framework:

```go
import (
    "encoding/json"
    "reverseProxy/internal/authorization"
    "reverseProxy/internal/jwtauth"
    "github.com/gofiber/fiber/v3"
)

func AuthorizationMiddleware(c fiber.Ctx) error {
    // 1. Extract request info
    method := c.Method()
    path := c.Path()
    fullURL := string(c.Request().URI().FullURI())
    
    // 2. Extract headers
    headers := make(map[string]string)
    c.Request().Header.VisitAll(func(key, value []byte) {
        headers[string(key)] = string(value)
    })
    
    // 3. Extract and parse request body
    var bodyData map[string]interface{}
    if err := json.Unmarshal(c.Body(), &bodyData); err != nil {
        // It's ok if body is not JSON (e.g., GET requests)
        bodyData = make(map[string]interface{})
    }
    
    // 4. Create RequestInfo
    req := authorization.RequestInfo{
        Method:  method,
        Path:    path,
        FullURL: fullURL,
        Headers: headers,
    }
    
    // 5. Get the authenticated principal (from JWT or other auth)
    principal := jwtauth.Principal{
        UserID:   c.Locals("userID").(string),
        Username: c.Locals("username").(string),
        Email:    c.Locals("email").(string),
    }
    
    // 6. Perform plainId authorization check
    allowed, reason, err := authorization.CheckPlainIdAccess(req, principal, bodyData)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Authorization service error",
            "detail": err.Error(),
        })
    }
    
    if !allowed {
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "Access denied",
            "reason": reason,
        })
    }
    
    return c.Next()
}

// Register middleware
app.Use(AuthorizationMiddleware)
```

## Configuration Details

### Basic Structure

```yaml
finegrain-check:
  enabled: true                                    # Enable/disable plainId checks
  validation-url: "http://plainid:8080/api/.."   # PlainId API endpoint
  client-id: "app-client-id"                      # Client credentials
  client-secret: "app-client-secret"
  client-auth-method: "client_secret_basic"       # Auth method (currently only this is supported)
  resource-map:                                   # Map of endpoint patterns to rules
    "[PATH:METHOD]": {rule}
```

### Resource Map Keys

Keys in the resource-map use the format: `[/path/pattern:METHOD]`

- Path patterns support wildcards:
  - `*` matches a single path segment
  - `**` matches remaining path segments
- Method is optional; if omitted, all methods match

Examples:
```yaml
"[/api/users:POST]"        # Exact path and method
"[/api/users/:id:GET]"     # Path with parameter pattern
"[/api/users/*:PUT]"       # Wildcard for one segment
"[/api/**]"                # Wildcard for remaining segments
"[/api/users]"             # No method specified, matches all methods
```

### Rule Configuration

Each rule specifies how to extract and authorize a request:

```yaml
rules:
  roles:           # Required roles (for reference)
    - "ROLE_USER"
  ruleset-name:   # PlainId ruleset name
    "transaction-policy"
  ruleset-id:     # PlainId ruleset ID
    "5001"
  body:           # Field extractions using JSON paths
    fieldName: $.jsonPath
    nested: $.parent.child[*].field
```

### JSON Path Patterns

Paths support multiple extraction patterns:

#### Simple Field
```yaml
body:
  username: $.username        # Extract $.username from request body
  email: $.email
```

#### Nested Fields
```yaml
body:
  userId: $.user.profile.id   # Navigate nested objects
  company: $.org.parent.name
```

#### Array Elements
```yaml
body:
  # Extract specific field from all array elements
  accountIds: $.accounts[*].id
  amounts: $.transfers[*].amount
```

#### Special: Existence Checks
Fields with "Used" or "Exists" in the name return `false` if the field is absent:

```yaml
body:
  # If $.templateId is present in request, this extracts it
  # If $.templateId is absent, this returns false
  templateUsed: $.templateId
```

## Advanced Usage

### Multiple Patterns for Similar Endpoints

```yaml
resource-map:
  "[/api/users:POST]":
    roles: ["ROLE_USER"]
    ruleset-name: "user-create"
    ruleset-id: "1001"
    body:
      username: $.username
      email: $.email

  "[/api/users/:id:PUT]":
    roles: ["ROLE_USER"]
    ruleset-name: "user-update"
    ruleset-id: "1002"
    body:
      userId: $.id
      email: $.email

  "[/api/users/*:DELETE]":
    roles: ["ROLE_ADMIN"]
    ruleset-name: "user-delete"
    ruleset-id: "1003"
    body:
      userId: $.id
```

### Complex Data Extraction

```yaml
"[/api/transfers:POST]":
  roles: ["ROLE_USER"]
  ruleset-name: "transfer-policy"
  ruleset-id: "2001"
  body:
    # Simple fields
    amount: $.amount
    currency: $.currency
    description: $.description
    
    # Nested fields
    fromAccountId: $.sourceAccount.id
    fromBankCode: $.sourceAccount.bank.code
    
    # Array fields - extract from all elements
    toAccountIds: $.recipients[*].accountId
    toAmounts: $.recipients[*].amount
    toBankCodes: $.recipients[*].bank.code
```

### Handler Implementation with Error Handling

```go
func TransferHandler(c fiber.Ctx) error {
    // Parse body
    var transferRequest struct {
        Amount      float64 `json:"amount"`
        Currency    string  `json:"currency"`
        Recipients  []map[string]interface{} `json:"recipients"`
    }
    
    if err := c.BindJSON(&transferRequest); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid request body",
        })
    }
    
    // Convert to generic map for plainId authorization
    bodyData := map[string]interface{}{
        "amount":     transferRequest.Amount,
        "currency":   transferRequest.Currency,
        "recipients": transferRequest.Recipients,
    }
    
    // Prepare request info
    headers := extractHeaders(c)
    req := authorization.RequestInfo{
        Method:  c.Method(),
        Path:    c.Path(),
        FullURL: string(c.Request().URI().FullURI()),
        Headers: headers,
    }
    
    // Get principal
    principal := getPrincipalFromContext(c)
    
    // Check authorization
    allowed, reason, err := authorization.CheckPlainIdAccess(req, principal, bodyData)
    
    if err != nil {
        log.Printf("Authorization check error: %v", err)
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Authorization failed",
        })
    }
    
    if !allowed {
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "Transfer not authorized",
            "reason": reason,
        })
    }
    
    // Proceed with transfer logic...
    return processTransfer(c, transferRequest)
}
```

## PlainId Response Handling

PlainId can respond with different decision types:

### Type 1: Explicit Permit
```json
{
  "permit": "PERMIT_EXPLICIT"
}
```
→ Access is **allowed**

### Type 2: Explicit Deny
```json
{
  "deny": "DENY_INSUFFICIENT_PRIVILEGES"
}
```
→ Access is **denied**

### Type 3: Standard Allow/Deny
```json
{
  "allow": true,
  "reason": "User has required role"
}
```
→ Decision based on `allow` field

## Debugging and Troubleshooting

### Enable Debug Logging

Add logging in your middleware:

```go
func AuthMiddlewareWithLogging(c fiber.Ctx) error {
    req := buildRequestInfo(c)
    principal := getPrincipal(c)
    bodyData := parseBody(c)
    
    log.Printf("Authorization check - Path: %s, Method: %s", req.Path, req.Method)
    log.Printf("Principal: %+v", principal)
    log.Printf("Body fields to check: %v", getFieldsForPath(req.Path))
    
    allowed, reason, err := authorization.CheckPlainIdAccess(req, principal, bodyData)
    
    log.Printf("Authorization result - Allowed: %v, Reason: %s, Error: %v", 
        allowed, reason, err)
    
    if !allowed {
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": reason,
        })
    }
    
    return c.Next()
}
```

### Common Issues and Solutions

#### Issue: "no matching rule"
**Solution**: Check that the request path and method match a pattern in the resource-map

#### Issue: "failed to extract field"
**Solution**: Verify the JSON path in the rule matches the actual request body structure

#### Issue: "non-2xx from plainId service"
**Solution**: Check plainId service is running and accessible at the configured validation-url

#### Issue: Request sent without expected fields
**Solution**: Verify the request body is valid JSON and contains the fields specified in the rules

## Testing

### Unit Test Example

```go
func TestTransferAuthorization(t *testing.T) {
    // Setup config
    cfg := &Config{
        FineGrain: FineGrainConfig{
            Enabled: true,
            ValidationURL: "http://mock-plainid:8080",
            ResourceMap: map[string]FineRule{
                "[/api/transfers:POST]": {
                    Body: map[string]string{
                        "amount": "$.amount",
                        "recipientId": "$.recipientId",
                    },
                },
            },
        },
    }
    
    // Test data
    req := RequestInfo{
        Method:  "POST",
        Path:    "/api/transfers",
        FullURL: "http://localhost/api/transfers",
        Headers: map[string]string{},
    }
    
    bodyData := map[string]interface{}{
        "amount": float64(1000),
        "recipientId": "recipient123",
    }
    
    principal := jwtauth.Principal{UserID: "user1", Username: "alice"}
    
    // Mock plainId response
    // ...
    
    // Verify result
    allowed, reason, err := authorization.CheckPlainIdAccess(req, principal, bodyData)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}
```

## Performance Considerations

- **HTTP Timeout**: The HTTP client has a 5-second timeout for plainId requests
- **Caching**: Consider implementing response caching for frequently checked endpoints
- **Fail-Open**: If no matching rule is found, access is allowed by default (fail-open policy)
- **Async Checks**: For high-traffic endpoints, consider making plainId checks asynchronous where appropriate

## References

- [PlainId API v5 Permit-Deny Endpoint](https://docs.plainid.io/apidocs/v5-permit-deny)
- [PlainId Policy Documentation](https://docs.plainid.io/apidocs/v5-endpoint-for-api-access)
- [Authorization Configuration Examples](./plainid-config-example.yaml)

