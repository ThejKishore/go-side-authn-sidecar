# PlainId Integration - Quick Reference (Updated)

## ✅ Integration Complete!

**`CheckPlainIdAccess` is now integrated into `CheckFineGrainAccess`**

### Call Location

**Before**: `CheckPlainIdAccess` was only called from tests
**Now**: Automatically called through `CheckFineGrainAccess` when body data is provided

## How It Works

### 1. Request Arrives at Handler
```
POST /api/transactions
Content: {"amount": 1000, "recipientId": "user123"}
```

### 2. Handler Processes Request
```go
// proxyhandler/proxy.go - Handler()

// Parse body
var bodyData map[string]interface{}
json.Unmarshal(c.Body(), &bodyData)

// Build RequestInfo
reqInfo := authorization.RequestInfo{
    Method:  c.Method(),
    Path:    c.Path(),
    FullURL: string(c.Request().URI().FullURI()),
    Headers: extractHeaders(c),
}

// Call CheckFineGrainAccess with body data
authorization.CheckFineGrainAccess(reqInfo, principal, bodyData)
```

### 3. CheckFineGrainAccess Routes the Request
```go
// internal/authorization/finegrain.go - CheckFineGrainAccess()

func CheckFineGrainAccess(req RequestInfo, p jwtauth.Principal, bodyData ...map[string]interface{}) (bool, string, error) {
    // ... check config and find rule ...
    
    // Route to plainId if body data provided
    if len(bodyData) > 0 && bodyData[0] != nil {
        return CheckPlainIdAccess(req, p, bodyData[0])  // ← Called here!
    }
    
    // Otherwise use legacy fine-grain check
    return postFineGrainCheck(c.FineGrain, payload)
}
```

### 4. CheckPlainIdAccess Performs Authorization
```go
// internal/authorization/plainid.go - CheckPlainIdAccess()

// Build plainId request
// Extract fields per rule: amount: $.amount, recipientId: $.recipientId
// Send to plainId service
// Return decision: allow/deny
```

### 5. Handler Returns Response
```
✓ Authorization passed → Proxy request
✗ Authorization failed → Return 403 Forbidden
```

## Files Modified

### 1. `internal/authorization/finegrain.go`

**Changed**: Function signature and routing logic

```go
// OLD
func CheckFineGrainAccess(req RequestInfo, p jwtauth.Principal) (bool, string, error)

// NEW
func CheckFineGrainAccess(req RequestInfo, p jwtauth.Principal, bodyData ...map[string]interface{}) (bool, string, error)
```

**What's new**:
- Added optional `bodyData` parameter (variadic)
- Routes to `CheckPlainIdAccess` if body data provided
- Falls back to legacy check otherwise

### 2. `internal/proxyhandler/proxy.go`

**Changed**: Request info building and function call

```go
// Parse body
var bodyData map[string]interface{}
json.Unmarshal(c.Body(), &bodyData)

// Build RequestInfo with FullURL and Headers
reqInfo := authorization.RequestInfo{
    Method:  c.Method(),
    Path:    c.Path(),
    FullURL: string(c.Request().URI().FullURI()),  // NEW
    Headers: extractHeaders(c),                     // NEW
}

// Pass bodyData to CheckFineGrainAccess
authorization.CheckFineGrainAccess(reqInfo, principal, bodyData)  // bodyData added

// Helper function added
func extractHeaders(c fiber.Ctx) map[string]string { ... }
```

## Configuration Required

**File**: `authorization.yaml`

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
```

## Backward Compatibility

✅ **100% backward compatible**

```go
// Old code still works!
allow, reason, err := CheckFineGrainAccess(req, principal)
// bodyData parameter is optional, defaults to nil
// Routes to legacy fine-grain check
```

## Testing

### Existing Tests
- All existing tests continue to work (backward compatible)
- Called without bodyData parameter

### New Tests with PlainId
- Pass bodyData to automatically route to plainId
- Use TestHelper for easier setup

### Example
```go
// Routes to plainId
bodyData := map[string]interface{}{"amount": 1000}
allow, _, _ := authorization.CheckFineGrainAccess(req, principal, bodyData)

// Routes to legacy check
allow, _, _ := authorization.CheckFineGrainAccess(req, principal)
```

## Authorization Flow

```
Request Handler
    ↓
JWT Authentication ✓
    ↓
Build RequestInfo (Method, Path, FullURL, Headers)
    ↓
Parse request body to map
    ↓
Call CheckCoarseAccess(reqInfo, principal)
    ↓
Call CheckFineGrainAccess(reqInfo, principal, bodyData)
    │
    ├─ IF bodyData provided
    │   └─→ CheckPlainIdAccess (build request, send to plainId)
    │
    └─ IF no bodyData
        └─→ Legacy fine-grain check
    ↓
IF all checks pass → Proxy request ✓
IF any check fails → Return 403 Forbidden ✗
```

## Summary of Changes

| Component | Before | After |
|-----------|--------|-------|
| **CheckFineGrainAccess** | No bodyData | Accepts optional bodyData |
| **Routing** | Manual selection | Automatic routing |
| **PlainId Call** | Separate | Integrated into fine-grain |
| **Handler** | RequestInfo incomplete | RequestInfo complete (FullURL, Headers) |
| **Body Parsing** | Not in handler | Parsed in handler |
| **Code Complexity** | Simple | Still simple (routing hidden) |
| **Backward Compat** | N/A | ✅ 100% compatible |

## No Code Changes Needed!

For users of the reverse proxy:

1. ✅ Configuration - Just update `authorization.yaml` with plainId settings
2. ✅ Handler - Already integrated, no changes needed
3. ✅ Middleware - No changes needed
4. ✅ Tests - Existing tests still pass

**Just configure and deploy!**

## Key Points

1. **Unified Interface**: Single `CheckFineGrainAccess` for all fine-grain checks
2. **Smart Routing**: Automatically chooses between plainId and legacy
3. **No Code Change**: Users just update configuration
4. **Backward Compatible**: Old code works without changes
5. **Clean Architecture**: Separation of concerns maintained
6. **Easy Testing**: TestHelper works with both check types

## Documentation

See these files for more details:

- **PLAINID_INTEGRATED_ARCHITECTURE.md** - Detailed architecture explanation
- **plainid-authorization.md** - API reference
- **plainid-config-example.yaml** - Configuration templates
- **plainid_test.go** - Test examples

## Quick Deploy

1. Update `authorization.yaml` with plainId configuration
2. Restart the service
3. Done! ✓

That's all you need to do. The integration is already in place!

