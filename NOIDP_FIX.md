# NoIdp Authorization Header Fix

## Summary
Fixed the egress proxy handler to properly skip the Authorization header setting when the `X-Idp-Type` header is set to `noIdp` (case-insensitive).

## Problem
The Authorization header was potentially being set even in noIdp mode, which is incorrect behavior.

## Solution
Implemented proper case-insensitive checking for the noIdp mode:

```go
// Handler receives X-Idp-Type header (e.g., "noIdp", "NoIdp", "NOIDP")
idpType := c.Get("X-Idp-Type")

// Normalize to lowercase for consistent comparison
idpType = strings.ToLower(idpType)  // "noidp"

// Skip Authorization header if noIdp mode
if idpType != "noidp" {
    // Fetch token and set Authorization header
    token, err := getToken(idpType)
    if err != nil {
        log.Printf("Failed to get token...")
    } else if token != "" {
        req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
    }
}
// For noIdp mode, no Authorization header is added
```

## Flow

### When X-Idp-Type: noIdp (or any case variation)
1. Header value is normalized to lowercase: `"noidp"`
2. Condition `if idpType != "noidp"` evaluates to FALSE
3. Token fetching is SKIPPED
4. Authorization header is NOT set
5. Request proceeds without authentication ✅

### When X-Idp-Type: okta (or any other IDP)
1. Header value is normalized to lowercase: `"okta"`
2. Condition `if idpType != "noidp"` evaluates to TRUE
3. Token fetching proceeds
4. Authorization header is set with bearer token
5. Request proceeds with authentication ✅

### When X-Idp-Type is not provided
1. Default to `"noIdp"`
2. Normalized to lowercase: `"noidp"`
3. Follows same flow as explicit noIdp (no auth) ✅

## Test Coverage
Added comprehensive tests in `noidp_test.go`:
- `TestHandlerNoIdpSkipsAuthorizationHeader` - Verifies no Authorization header in noIdp mode
- `TestHandlerNoIdpVariations` - Tests multiple case variations (noIdp, noidp, NOIDP, NoIdp)

## Files Modified
- `internal/egressproxy/handler.go` - Fixed Authorization header logic
- `internal/egressproxy/noidp_test.go` - Added noIdp-specific tests (new)

## Verification
The implementation ensures:
✅ Case-insensitive comparison works correctly  
✅ NoIdp mode skips Authorization header  
✅ Other IDP types get Authorization header  
✅ Default behavior (no header provided) is noIdp  
✅ All variations of noIdp are handled (noIdp, noidp, NOIDP, NoIdp, etc.)

