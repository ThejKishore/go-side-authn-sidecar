# CheckPlainIdAccess Integration Guide

## Quick Answer

**`CheckPlainIdAccess` is currently NOT called from any production code.**

It is only called from:
- **9 test functions** in `plainid_test.go`
- **1 test utility** in `plainid_testhelper.go`

## Where It Should Be Called

**Location**: `internal/proxyhandler/proxy.go` → `Handler()` function

**Current Flow**:
1. JWT authentication
2. Coarse-grain authorization check
3. Fine-grain authorization check
4. Proxy the request

**Required Change**:
Add `CheckPlainIdAccess` call after JWT authentication, alongside other authorization checks.

## What Needs to Change

### 1. Update RequestInfo Structure

**Current**:
```go
reqInfo := authorization.RequestInfo{
    Method: c.Method(),
    Path:   c.OriginalURL(),
}
```

**Required**:
```go
reqInfo := authorization.RequestInfo{
    Method:  c.Method(),
    Path:    c.Path(),
    FullURL: string(c.Request().URI().FullURI()),  // NEW
    Headers: extractHeaders(c),                     // NEW
}
```

### 2. Add Helper Function

Add this function to `proxyhandler/proxy.go`:

```go
func extractHeaders(c fiber.Ctx) map[string]string {
    headers := make(map[string]string)
    c.Request().Header.VisitAll(func(key, value []byte) {
        headers[string(key)] = string(value)
    })
    return headers
}
```

### 3. Parse Request Body

Add to `Handler()`:

```go
var bodyData map[string]interface{}
if err := json.Unmarshal(c.Body(), &bodyData); err != nil {
    bodyData = make(map[string]interface{})
}
```

### 4. Add Authorization Check

Add to `Handler()` after JWT authentication:

```go
go func() {
    allow, reason, err := authorization.CheckPlainIdAccess(reqInfo, principal, bodyData)
    plainIdCh <- authResult{allow: allow, reason: reason, err: err}
}()
```

### 5. Validate Result

After getting all results:

```go
plainIdRes := <-plainIdCh

if plainIdRes.err != nil {
    return fiber.NewError(fiber.StatusForbidden, "plainId authorization error: "+plainIdRes.err.Error())
}
if !plainIdRes.allow {
    reason := plainIdRes.reason
    if reason == "" {
        reason = "plainId authorization denied"
    }
    return fiber.NewError(fiber.StatusForbidden, reason)
}
```

## Configuration Required

Update `authorization.yaml`:

```yaml
finegrain-check:
  enabled: true
  validation-url: "http://plainid:8080/fga/api/runtime/5.0/decisions/permit-deny"
  client-id: "your-client-id"
  client-secret: "your-client-secret"
  client-auth-method: "client_secret_basic"
  
  resource-map:
    "[/api/endpoint:POST]":
      roles: ["ROLE_USER"]
      ruleset-name: "endpoint-policy"
      ruleset-id: "1001"
      body:
        field1: $.field1
        field2: $.field2[*].id
```

## Current Call Locations

### Test Files
- `plainid_test.go:287` - TestCheckPlainIdAccess_Allow
- `plainid_test.go:339` - TestCheckPlainIdAccess_Deny
- `plainid_test.go:363` - TestCheckPlainIdAccess_SkipWhenDisabled
- `plainid_test.go:396` - TestCheckPlainIdAccess_SkipWhenNoMatchingRule
- `plainid_test.go:437` - TestCheckPlainIdAccess_PlainIdPermit
- `plainid_test.go:478` - TestCheckPlainIdAccess_PlainIdDeny
- `plainid_test.go:649` - TestCheckPlainIdAccess_UsingTestHelper
- `plainid_test.go:696` - TestCheckPlainIdAccess_TestHelperWithDeny
- `plainid_test.go:727` - TestCheckPlainIdAccess_TestHelperArrayExtraction

### Test Helper
- `plainid_testhelper.go:151` - TestHelper.CheckAccess()

### Production Code
- **NONE** - Not yet integrated

## Integration Steps

1. **Review** documentation/plainid-usage-guide.md
2. **Modify** internal/proxyhandler/proxy.go
3. **Update** authorization.yaml with plainId config
4. **Test** with: `go test ./internal/authorization -v`
5. **Deploy** with plainId service

## Key Files

**Implementation**:
- `internal/authorization/plainid.go` - CheckPlainIdAccess function

**Where to integrate**:
- `internal/proxyhandler/proxy.go` - Handler() function

**Tests**:
- `internal/authorization/plainid_test.go` - 9 test examples
- `internal/authorization/plainid_testhelper.go` - Testing utilities

**Documentation**:
- `documentation/plainid-usage-guide.md` - Integration guide
- `documentation/plainid-authorization.md` - API reference
- `documentation/PLAINID_CALL_LOCATION.md` - This analysis

## Testing

Use the TestHelper for testing integration:

```go
func TestMyIntegration(t *testing.T) {
    helper := authorization.NewTestHelper(t)
    defer helper.Close()
    
    helper.SetupConfig(map[string]authorization.FineRule{
        "[/api/test:POST]": {
            Body: map[string]string{"id": "$.id"},
        },
    })
    
    allowed, _, _ := helper.CheckAccess(
        "POST", "/api/test", "https://api.example.com/api/test",
        headers, bodyData,
    )
    
    if !allowed {
        t.Error("expected access to be allowed")
    }
}
```

## Summary

| Aspect | Current | Required |
|--------|---------|----------|
| **Function Defined** | ✓ Yes | - |
| **Tests Written** | ✓ Yes (19) | - |
| **Test Helper** | ✓ Yes | - |
| **Called from Tests** | ✓ Yes | - |
| **Called from Production** | ❌ No | ✓ Add to Handler() |
| **Configuration** | Optional | Required |
| **RequestInfo Updated** | Partial | Full |
| **Body Parsing** | Not done | Required |
| **Header Extraction** | Not done | Required |

## Next Action

**Open** `internal/proxyhandler/proxy.go` and:
1. Add the helper function `extractHeaders()`
2. Update `RequestInfo` creation
3. Add body parsing
4. Add plainId authorization check
5. Validate result before proxying

See `documentation/plainid-usage-guide.md` for detailed code examples.

