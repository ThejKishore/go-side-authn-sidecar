# PlainId Authorization Implementation

## Overview

The plainId authorization class provides fine-grained access control integration with the plainId service. It enables the application to authorize requests based on policies defined in plainId while extracting request context and body parameters according to configuration specifications.

## Components

### Core Structures

#### `PlainIdRequest`
Represents the request sent to the plainId API for authorization decisions.

```go
type PlainIdRequest struct {
    Method  string                 // HTTP method (e.g., "POST", "GET")
    Headers map[string]string      // Request headers (x-request-id, Authorization)
    URI     PlainIdURI             // URI components (schema, authority, path, query)
    Body    map[string]interface{} // Extracted body parameters
    Meta    PlainIdMeta            // Runtime metadata
}
```

#### `PlainIdURI`
Contains URI components parsed from the incoming request.

```go
type PlainIdURI struct {
    Schema    string                     // URL schema (http, https)
    Authority map[string]string          // Authority parameters (host, port)
    Path      []string                   // Path segments, including full path
    Query     map[string]interface{}     // Query parameters
}
```

#### `PlainIdResponse`
The response structure from the plainId API.

```go
type PlainIdResponse struct {
    Allow  bool   // Default allow/deny decision
    Permit string // Explicit PERMIT result (takes precedence)
    Deny   string // Explicit DENY result (takes precedence)
    Reason string // Reason for the decision
}
```

### Main Functions

#### `CheckPlainIdAccess`
Primary entry point for plainId authorization checks.

```go
func CheckPlainIdAccess(req RequestInfo, p jwtauth.Principal, bodyData map[string]interface{}) (bool, string, error)
```

**Parameters:**
- `req` - The incoming request information (method, path, full URL, headers)
- `p` - The authenticated principal/user information
- `bodyData` - The parsed request body as a map

**Returns:**
- `bool` - Whether access is allowed
- `string` - Reason for the decision
- `error` - Any error that occurred during the check

**Behavior:**
1. If plainId is disabled or no validation URL is configured, returns allow=true
2. Looks for a matching rule in the resource map based on method and path
3. If no rule matches, returns allow=true (fail-open)
4. Builds a plainId request from the incoming request and rule configuration
5. Sends the request to the plainId service
6. Parses and returns the response

#### `buildPlainIdRequest`
Constructs a PlainIdRequest from incoming request data and configuration.

```go
func buildPlainIdRequest(req RequestInfo, p jwtauth.Principal, rule FineRule, bodyData map[string]interface{}) (PlainIdRequest, error)
```

This function:
- Parses the full URL to extract schema, host, port, and query parameters
- Builds the path array with full path and segment parts
- Extracts header values, preserving x-request-id and Authorization
- Extracts body fields using JSON paths defined in the rule
- Returns a properly formatted PlainIdRequest

#### `extractBodyFromRule`
Extracts body field values according to the rule configuration.

```go
func extractBodyFromRule(bodyData map[string]interface{}, rule FineRule) (map[string]interface{}, error)
```

For each field defined in the rule's body map, this function:
1. Gets the JSON path (e.g., "$.transactionName")
2. Extracts the value from the request body
3. Returns all extracted fields as a map

### JSON Path Extraction Functions

#### `extractValueFromPath`
Extracts a value from a JSON object using a JSON path.

**Supported Patterns:**
- **Simple paths**: `$.fieldName` or `fieldName`
- **Nested paths**: `$.parent.child.grandchild`
- **Array wildcards**: `$.array[*].field` - extracts the field from all array elements
- **Existence checks**: Fields containing "Used" or "Exists" in the name return false if the field is not found

**Examples:**
```go
// Simple field extraction
path: "$.username"
data: {"username": "alice"}
result: "alice"

// Nested field extraction
path: "$.user.profile.name"
data: {"user": {"profile": {"name": "Bob"}}}
result: "Bob"

// Array wildcard extraction
path: "$.accounts[*].accountId"
data: {"accounts": [{"accountId": "123"}, {"accountId": "456"}]}
result: ["123", "456"]
```

#### `extractArrayWildcard`
Specialized function for extracting values from array elements using the `[*]` wildcard pattern.

## Configuration

The plainId authorization is configured in the `authorization.yaml` file under the `finegrain-check` section:

```yaml
finegrain-check:
  enabled: true
  validation-url: "http://localhost:8080/fga/api/runtime/5.0/decisions/permit-deny"
  client-id: "plt-client"
  client-secret: "plt-secret"
  client-auth-method: "client_secret_basic"
  resource-map:
    "[/mm/web/v1/transaction:POST]":
      roles: ["ROLE_USER"]
      ruleset-name: "mm-transaction"
      ruleset-id: "10201"
      body:
        transactionName: $.transactionName
        transactionAmount: $.transactionAmount
        tranTemplateUsed: $.tranTemplateID  # Boolean: true if present, false otherwise
        fromAccountIds: $.fromAccount[*].accountId
        toAccountIds: $.toAccount[*].accountId
        fromAccountValues: $.fromAccount[*].accountValue
```

### Configuration Fields

- **enabled**: Whether plainId authorization is active
- **validation-url**: The plainId API endpoint for permit/deny decisions
- **client-id**: Client ID for authentication to plainId service
- **client-secret**: Client secret for authentication (used with client_secret_basic)
- **client-auth-method**: Authentication method (currently supports "client_secret_basic")
- **resource-map**: Maps request patterns to authorization rules
  - Key format: `[/path/pattern:METHOD]`
  - Value: Rule configuration with roles, ruleset info, and body field mappings

### Rule Configuration

Each rule in the resource-map specifies:
- **roles**: Required roles for this endpoint
- **ruleset-name**: Name of the ruleset in plainId
- **ruleset-id**: ID of the ruleset in plainId
- **body**: JSON path mappings for extracting body fields

## Example Usage

### In Middleware

```go
// Extract request body
var bodyData map[string]interface{}
if err := json.Unmarshal(c.Body(), &bodyData); err != nil {
    // Handle error
}

// Create RequestInfo
req := RequestInfo{
    Method:  c.Method(),
    Path:    c.Path(),
    FullURL: string(c.Request().URI().FullURI()),
    Headers: extractHeaders(c),
}

// Check plainId authorization
allow, reason, err := CheckPlainIdAccess(req, principal, bodyData)
if err != nil {
    return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
        "error": "Authorization check failed",
    })
}

if !allow {
    return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
        "error": reason,
    })
}
```

### Request to PlainId Service

The application sends a POST request to the plainId validation URL with a JSON body containing:

```json
{
  "method": "POST",
  "headers": {
    "x-request-id": "8CDAC3e6r4D252ABE60EFD7A31AFEEBA",
    "Authorization": "Bearer eyJhbG...lXvZQ"
  },
  "uri": {
    "schema": "https",
    "authority": {
      "host": "localhost",
      "port": "8080"
    },
    "path": [
      "/mm/web/v1/transaction",
      "mm",
      "web",
      "v1",
      "transaction"
    ],
    "query": {
      "details": "true"
    }
  },
  "body": {
    "transactionName": "Test",
    "transactionAmount": "100",
    "tranTemplateUsed": true,
    "fromAccountIds": ["1234567890", "1234567891", "1234567892"],
    "toAccountIds": ["1234567893", "1234567894", "1234567895"],
    "fromAccountValues": ["10", "80", "10"]
  },
  "meta": {
    "runtimeFineTune": {
      "combinedMultiValue": false
    }
  }
}
```

## Response Handling

PlainId can respond with three different decision mechanisms:

1. **Explicit Permit**: `{"permit": "PERMIT_EXPLICIT"}` → Access allowed
2. **Explicit Deny**: `{"deny": "DENY_POLICY_VIOLATION"}` → Access denied
3. **Allow/Deny Flag**: `{"allow": true/false, "reason": "..."}` → Standard allow/deny decision

The implementation checks in this order:
1. If `permit` is set → allow access
2. If `deny` is set → deny access
3. Otherwise, use the `allow` flag

## Error Handling

The implementation handles errors in the following ways:

- **Configuration missing**: Returns allow=true with skip reason
- **No matching rule**: Returns allow=true with skip reason (fail-open)
- **Invalid JSON path**: Returns error to prevent authorization
- **PlainId service errors**: Returns error with appropriate message
- **Non-2xx response from plainId**: Returns deny with "non-2xx from plainId service" reason

## Testing

Comprehensive test coverage includes:

- Simple field extraction
- Nested field extraction
- Array wildcard extraction
- Full PlainIdRequest building
- Authorization check scenarios (allow, deny, skip)
- PlainId-specific response handling (Permit, Deny)
- Error conditions

Run tests with:
```bash
go test ./internal/authorization -v
```

## References

- [PlainId API v5 Contract](https://docs.plainid.io/apidocs/v5-permit-deny)
- [PlainId API v5 Documentation](https://docs.plainid.io/apidocs/v5-endpoint-for-api-access)

