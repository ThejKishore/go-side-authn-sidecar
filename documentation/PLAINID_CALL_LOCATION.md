# CheckPlainIdAccess - Call Location Analysis

## Current Status

**`CheckPlainIdAccess` is NOT currently being called anywhere in the production code.**

It is only called from:
1. **Test files** (`plainid_test.go`) - 9 direct calls in unit tests
2. **Test helper** (`plainid_testhelper.go`) - 1 call from `TestHelper.CheckAccess()`

## Where It Should Be Called

The function should be integrated into the **authorization middleware** in the reverse proxy handler.

### Current Flow in `internal/proxyhandler/proxy.go`

```go
func Handler(c fiber.Ctx) error {
    // 1. JWT Authentication
    jwtError, isJwtError := jwtAuthenticate(c)
    
    // 2. Extract Principal
    principal, _ := c.Locals("Principal").(jwtauth.Principal)
    
    // 3. Create RequestInfo
    reqInfo := authorization.RequestInfo{
        Method: c.Method(),
        Path:   c.OriginalURL(),
    }
    
    // 4. Run coarse and fine-grain authorization
    go func() {
        allow, reason, err := authorization.CheckCoarseAccess(reqInfo, principal)
        coarseCh <- authResult{allow: allow, reason: reason, err: err}
    }()
    
    go func() {
        allow, reason, err := authorization.CheckFineGrainAccess(reqInfo, principal)
        fineCh <- authResult{allow: allow, reason: reason, err: err}
    }()
    
    // 5. Proxy the request
    target := "https://httpbin.org" + c.OriginalURL()
    return doProxy(c, target)
}
```

## How to Integrate CheckPlainIdAccess

There are two approaches:

### Option 1: Add plainId Check to the Handler (Recommended)

Integrate `CheckPlainIdAccess` as an additional authorization step alongside coarse and fine-grain checks:

```go
func Handler(c fiber.Ctx) error {
    // JWT Authentication
    jwtError, isJwtError := jwtAuthenticate(c)
    if isJwtError {
        return jwtError
    }
    
    principal, _ := c.Locals("Principal").(jwtauth.Principal)
    
    // Build RequestInfo with all required fields
    reqInfo := authorization.RequestInfo{
        Method:  c.Method(),
        Path:    c.Path(),
        FullURL: string(c.Request().URI().FullURI()),  // NEW: Required for plainId
        Headers: extractHeaders(c),                      // NEW: Extract headers
    }
    
    // Parse request body for plainId authorization
    var bodyData map[string]interface{}
    if err := json.Unmarshal(c.Body(), &bodyData); err != nil {
        bodyData = make(map[string]interface{})
    }
    
    // Run all three authorization checks
    type authResult struct {
        allow  bool
        reason string
        err    error
    }
    
    coarseCh := make(chan authResult, 1)
    fineCh := make(chan authResult, 1)
    plainIdCh := make(chan authResult, 1)
    
    // Coarse-grain check
    go func() {
        allow, reason, err := authorization.CheckCoarseAccess(reqInfo, principal)
        coarseCh <- authResult{allow: allow, reason: reason, err: err}
    }()
    
    // Fine-grain check
    go func() {
        allow, reason, err := authorization.CheckFineGrainAccess(reqInfo, principal)
        fineCh <- authResult{allow: allow, reason: reason, err: err}
    }()
    
    // PlainId check (NEW)
    go func() {
        allow, reason, err := authorization.CheckPlainIdAccess(reqInfo, principal, bodyData)
        plainIdCh <- authResult{allow: allow, reason: reason, err: err}
    }()
    
    // Wait and validate all results
    coarseRes := <-coarseCh
    fineRes := <-fineCh
    plainIdRes := <-plainIdCh
    
    // Check coarse
    if coarseRes.err != nil {
        return fiber.NewError(fiber.StatusForbidden, "coarse authorization error: "+coarseRes.err.Error())
    }
    if !coarseRes.allow {
        reason := coarseRes.reason
        if reason == "" {
            reason = "coarse authorization denied"
        }
        return fiber.NewError(fiber.StatusForbidden, reason)
    }
    
    // Check fine-grain
    if fineRes.err != nil {
        return fiber.NewError(fiber.StatusForbidden, "fine-grain authorization error: "+fineRes.err.Error())
    }
    if !fineRes.allow {
        reason := fineRes.reason
        if reason == "" {
            reason = "fine-grain authorization denied"
        }
        return fiber.NewError(fiber.StatusForbidden, reason)
    }
    
    // Check plainId (NEW)
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
    
    // All checks passed - proxy the request
    target := "https://httpbin.org" + c.OriginalURL()
    return doProxy(c, target)
}
```

### Option 2: Create a Separate plainId Middleware

Alternatively, create a dedicated middleware for plainId checks:

```go
func PlainIdAuthMiddleware(c fiber.Ctx) error {
    principal, ok := c.Locals("Principal").(jwtauth.Principal)
    if !ok {
        return c.Next()
    }
    
    // Extract request info
    var bodyData map[string]interface{}
    if err := json.Unmarshal(c.Body(), &bodyData); err != nil {
        bodyData = make(map[string]interface{})
    }
    
    reqInfo := authorization.RequestInfo{
        Method:  c.Method(),
        Path:    c.Path(),
        FullURL: string(c.Request().URI().FullURI()),
        Headers: extractHeaders(c),
    }
    
    // Check plainId authorization
    allowed, reason, err := authorization.CheckPlainIdAccess(reqInfo, principal, bodyData)
    
    if err != nil {
        return fiber.NewError(fiber.StatusForbidden, "plainId authorization error: "+err.Error())
    }
    
    if !allowed {
        return fiber.NewError(fiber.StatusForbidden, "plainId authorization denied: "+reason)
    }
    
    return c.Next()
}

// In main.go:
app.Use(PlainIdAuthMiddleware)
app.All("/*", proxyhandler.Handler)
```

## Helper Function Needed

Both approaches require a helper function to extract headers from the Fiber context:

```go
func extractHeaders(c fiber.Ctx) map[string]string {
    headers := make(map[string]string)
    c.Request().Header.VisitAll(func(key, value []byte) {
        headers[string(key)] = string(value)
    })
    return headers
}
```

## Summary

| Call Location | Count | Type |
|---------------|-------|------|
| `plainid.go` | 1 | Function definition |
| `plainid_test.go` | 9 | Direct test calls |
| `plainid_testhelper.go` | 1 | Test utility call |
| **Production code** | **0** | ❌ NOT INTEGRATED YET |

## Next Steps

To integrate `CheckPlainIdAccess` into production:

1. **Update `proxyhandler/proxy.go`**:
   - Add `FullURL` to RequestInfo
   - Add `Headers` extraction to RequestInfo
   - Parse request body to `map[string]interface{}`
   - Add plainId authorization check

2. **Add helper function**:
   - `extractHeaders()` to extract headers from context

3. **Update `authorization.yaml`**:
   - Configure plainId endpoints with rules
   - Set validation URL, credentials, etc.

4. **Test the integration**:
   - Write integration tests
   - Use TestHelper for testing
   - Verify plainId service connectivity

See `documentation/plainid-usage-guide.md` for detailed integration instructions.

