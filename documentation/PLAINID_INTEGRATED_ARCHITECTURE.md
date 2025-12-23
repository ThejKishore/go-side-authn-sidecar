# PlainId Authorization - Integrated into CheckFineGrainAccess

## Architecture

The `CheckPlainIdAccess` function has been integrated directly into `CheckFineGrainAccess` method. This is the correct approach because:

1. **PlainId is a fine-grained authorization mechanism** - It performs detailed authorization checks based on request context
2. **Unified fine-grain interface** - Both legacy fine-grain checks and plainId use the same `CheckFineGrainAccess` function
3. **Automatic routing** - When body data is provided, the method automatically routes to plainId
4. **Backward compatible** - Existing code that calls `CheckFineGrainAccess` without body data continues to work

## Call Flow

```
Request arrives at proxy Handler
    ↓
JWT Authentication
    ↓
Build RequestInfo (with FullURL, Headers)
    ↓
Parse request body to map[string]interface{}
    ↓
Call CheckFineGrainAccess(reqInfo, principal, bodyData)
    ↓
CheckFineGrainAccess receives bodyData:
  ├─ IF bodyData provided → Route to CheckPlainIdAccess
  └─ ELSE → Use legacy fine-grain check
    ↓
Return authorization decision (allow/deny/error)
    ↓
IF allowed → Proxy request
IF denied → Return 403 Forbidden
```

## Implementation Details

### 1. Updated CheckFineGrainAccess Signature

**File**: `internal/authorization/finegrain.go`

```go
func CheckFineGrainAccess(req RequestInfo, p jwtauth.Principal, bodyData ...map[string]interface{}) (bool, string, error)
```

**Parameters**:
- `req` - RequestInfo (method, path, fullURL, headers)
- `p` - Principal (authenticated user)
- `bodyData` - Optional variadic parameter containing request body as map

**Logic**:
1. Check if configuration exists and is enabled
2. Find matching rule for the request path/method
3. If body data is provided → call `CheckPlainIdAccess`
4. Otherwise → use legacy fine-grain check

**Code**:
```go
func CheckFineGrainAccess(req RequestInfo, p jwtauth.Principal, bodyData ...map[string]interface{}) (bool, string, error) {
    c := ConfigOrNil()
    if c == nil || !c.FineGrain.Enabled || c.FineGrain.ValidationURL == "" {
        return true, "fine-grain check skipped (no config)", nil
    }
    rule, ok := c.FineGrain.MatchRule(req.Method, req.Path)
    if !ok {
        return true, "fine-grain check skipped (no matching rule)", nil
    }

    // If body data is provided, use plainId authorization
    if len(bodyData) > 0 && bodyData[0] != nil {
        return CheckPlainIdAccess(req, p, bodyData[0])
    }

    // Fall back to legacy fine-grain check
    payload := finePayload{
        Principal: p,
        Request:   req,
        Rule:      rule,
    }
    return postFineGrainCheck(c.FineGrain, payload)
}
```

### 2. Updated Handler in ProxyHandler

**File**: `internal/proxyhandler/proxy.go`

```go
func Handler(c fiber.Ctx) error {
    // ... JWT authentication ...

    principal, _ := c.Locals("Principal").(jwtauth.Principal)

    // Parse request body for authorization checks
    var bodyData map[string]interface{}
    if err := json.Unmarshal(c.Body(), &bodyData); err != nil {
        bodyData = make(map[string]interface{})
    }

    // Build RequestInfo with full details
    reqInfo := authorization.RequestInfo{
        Method:  c.Method(),
        Path:    c.Path(),
        FullURL: string(c.Request().URI().FullURI()),
        Headers: extractHeaders(c),
    }

    // Pass bodyData to CheckFineGrainAccess
    // It will automatically route to plainId if body data is provided
    go func() {
        allow, reason, err := authorization.CheckFineGrainAccess(reqInfo, principal, bodyData)
        fineCh <- authResult{allow: allow, reason: reason, err: err}
    }()

    // ... rest of the handler ...
}
```

### 3. Helper Function

**File**: `internal/proxyhandler/proxy.go`

```go
func extractHeaders(c fiber.Ctx) map[string]string {
    headers := make(map[string]string)
    c.Request().Header.VisitAll(func(key, value []byte) {
        headers[string(key)] = string(value)
    })
    return headers
}
```

## Configuration

Update `authorization.yaml` to enable plainId fine-grained checks:

```yaml
finegrain-check:
  enabled: true
  validation-url: "http://plainid:8080/fga/api/runtime/5.0/decisions/permit-deny"
  client-id: "app-client"
  client-secret: "app-secret"
  client-auth-method: "client_secret_basic"
  
  resource-map:
    "[/api/transactions:POST]":
      roles: ["ROLE_USER"]
      ruleset-name: "transaction-policy"
      ruleset-id: "1001"
      body:
        amount: $.amount
        recipientId: $.recipientId
        accountIds: $.accounts[*].id
```

## Request Flow Example

### 1. Incoming Request
```
POST /api/transactions
Authorization: Bearer <jwt-token>
Content-Type: application/json

{
  "amount": 1000,
  "recipientId": "user123",
  "accounts": [{"id": "acc1"}, {"id": "acc2"}]
}
```

### 2. Handler Processing
```
JWT Authentication ✓
Build RequestInfo:
  - Method: POST
  - Path: /api/transactions
  - FullURL: http://localhost:3001/api/transactions
  - Headers: {Authorization: Bearer..., Content-Type: application/json}

Parse body:
  - amount: 1000
  - recipientId: user123
  - accounts: [{id: acc1}, {id: acc2}]

Call CheckFineGrainAccess(reqInfo, principal, bodyData)
```

### 3. CheckFineGrainAccess Routing
```
Configuration loaded ✓
Rule found: [/api/transactions:POST] ✓
Body data provided ✓

Route to CheckPlainIdAccess(reqInfo, principal, bodyData)
```

### 4. CheckPlainIdAccess Processing
```
Build plainId request with:
  - URI components (schema, host, port, path, query)
  - Headers (x-request-id, Authorization)
  - Body fields extracted per rule:
    - amount: 1000
    - recipientId: user123
    - accountIds: [acc1, acc2]

Send to plainId service
  POST http://plainid:8080/fga/api/runtime/5.0/decisions/permit-deny
  
Receive response:
  {"permit": "PERMIT_EXPLICIT"} or {"allow": true}

Return (allow=true, reason="PERMIT_EXPLICIT", err=nil)
```

### 5. Handler Response
```
All authorization checks passed ✓
Proxy request to backend ✓
Return 200 OK with backend response
```

## Migration Path

### For Existing Code

**Old code** (without body data):
```go
allow, reason, err := authorization.CheckFineGrainAccess(req, principal)
```

**Still works** - backward compatible! The bodyData parameter is optional.

### For New Code with PlainId

**New code** (with body data for plainId):
```go
var bodyData map[string]interface{}
json.Unmarshal(c.Body(), &bodyData)

allow, reason, err := authorization.CheckFineGrainAccess(req, principal, bodyData)
```

## Testing

### Unit Tests

Existing tests in `finegrain_test.go` work as-is (backward compatible):

```go
// Without body data - uses legacy check
allow, reason, err := CheckFineGrainAccess(req, principal)
```

### Integration Tests with PlainId

New tests with body data route to plainId:

```go
// With body data - routes to plainId
bodyData := map[string]interface{}{
    "amount": 1000,
    "recipientId": "user123",
}

allow, reason, err := CheckFineGrainAccess(req, principal, bodyData)
```

### Using TestHelper

```go
helper := authorization.NewTestHelper(t)
defer helper.Close()

helper.SetupConfig(map[string]authorization.FineRule{
    "[/api/transactions:POST]": {
        Body: map[string]string{"amount": "$.amount"},
    },
})

allowed, _, _ := helper.CheckAccess(
    "POST", "/api/transactions", "https://api.example.com/api/transactions",
    headers, bodyData,
)
```

## Advantages of This Approach

1. **Single entry point** - All fine-grained checks go through `CheckFineGrainAccess`
2. **Backward compatible** - Existing code without body data still works
3. **Clean separation** - PlainId check is isolated in `CheckPlainIdAccess`
4. **Flexible routing** - Automatically chooses between legacy and plainId
5. **Consistent API** - Same function signature for both check types
6. **Easy testing** - TestHelper works with both legacy and plainId
7. **Maintainable** - Clear flow and logic

## Error Handling

### Scenario 1: Invalid Configuration
```
CheckFineGrainAccess called
Configuration missing/disabled
→ Return (allow=true, reason="skipped", err=nil)
→ Request allowed (fail-open)
```

### Scenario 2: No Matching Rule
```
CheckFineGrainAccess called
No rule matches path/method
→ Return (allow=true, reason="skipped", err=nil)
→ Request allowed (fail-open)
```

### Scenario 3: PlainId Check Passes
```
CheckPlainIdAccess called
PlainId responds with permit/allow=true
→ Return (allow=true, reason="...", err=nil)
→ Request proxied
```

### Scenario 4: PlainId Check Fails
```
CheckPlainIdAccess called
PlainId responds with deny/allow=false
→ Return (allow=false, reason="...", err=nil)
→ Handler returns 403 Forbidden
```

### Scenario 5: PlainId Service Error
```
CheckPlainIdAccess called
PlainId service unavailable or error
→ Return (allow=false, reason="...", err=<error>)
→ Handler returns 403 Forbidden with error
```

## Files Modified

1. **internal/authorization/finegrain.go**
   - Updated `CheckFineGrainAccess` signature to accept optional bodyData
   - Added routing logic to call `CheckPlainIdAccess` when body data is provided

2. **internal/proxyhandler/proxy.go**
   - Updated `RequestInfo` to include `FullURL` and `Headers`
   - Added body parsing with `json.Unmarshal`
   - Updated call to `CheckFineGrainAccess` to pass `bodyData`
   - Added `extractHeaders` helper function

## Backward Compatibility

✅ **Fully backward compatible**:
- Existing code calling `CheckFineGrainAccess(req, principal)` continues to work
- bodyData parameter is optional (variadic)
- No changes to coarse-grain authorization
- Existing configuration still valid

## Next Steps

1. ✅ Integration complete
2. ✅ Tests passing
3. ✅ Documentation updated
4. Deploy with plainId configuration in `authorization.yaml`
5. Monitor authorization logs
6. Test with real plainId service

## References

- Implementation: `internal/authorization/plainid.go`
- Integration: `internal/authorization/finegrain.go`
- Handler: `internal/proxyhandler/proxy.go`
- Tests: `internal/authorization/plainid_test.go`
- Configuration: `authorization.yaml`

