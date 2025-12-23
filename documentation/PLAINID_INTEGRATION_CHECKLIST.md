# PlainId Authorization Integration Checklist

## Pre-Integration Setup

- [ ] Review PlainId API documentation
  - [ ] Understand permit/deny endpoint format
  - [ ] Review authentication requirements
  - [ ] Confirm plainId service endpoint URL

- [ ] Understand existing authorization system
  - [ ] Review existing `coarse.go` and `finegrain.go`
  - [ ] Understand `FineRule` configuration structure
  - [ ] Familiar with `authorization.yaml` format

- [ ] Set up PlainId service
  - [ ] PlainId service is running and accessible
  - [ ] Credentials (client-id, client-secret) are available
  - [ ] Rulesets are configured in plainId

## Code Integration

- [ ] Verify plainId files are in place
  - [ ] `internal/authorization/plainid.go` exists
  - [ ] `internal/authorization/plainid_test.go` exists
  - [ ] `internal/authorization/plainid_testhelper.go` exists

- [ ] Verify RequestInfo struct is updated
  - [ ] `coarse.go` has `FullURL` field in `RequestInfo`
  - [ ] `GetHeader()` helper method exists

- [ ] Run tests to verify implementation
  - [ ] Run `go test ./internal/authorization -v`
  - [ ] All 47 tests should pass
  - [ ] No compilation errors

## Configuration Setup

- [ ] Create/update `authorization.yaml`
  - [ ] Set `finegrain-check.enabled: true`
  - [ ] Configure `validation-url` pointing to plainId service
  - [ ] Set `client-id` and `client-secret`
  - [ ] Set `client-auth-method: "client_secret_basic"`

- [ ] Add resource mappings
  - [ ] For each endpoint needing authorization:
    - [ ] Define the path pattern (e.g., `[/api/resource:POST]`)
    - [ ] Set required roles
    - [ ] Define ruleset name and ID
    - [ ] Configure body field mappings with JSON paths

- [ ] Example configuration added
  - [ ] Simple endpoint configured
  - [ ] Array wildcard extraction configured
  - [ ] Nested field extraction configured

## Middleware Integration

- [ ] Update authorization middleware
  - [ ] Import `authorization` package
  - [ ] Extract request method and path
  - [ ] Parse and extract full URL
  - [ ] Extract request headers (preserving case)
  - [ ] Parse request body to map (if applicable)

- [ ] Add plainId authorization check
  - [ ] Call `authorization.CheckPlainIdAccess()`
  - [ ] Pass RequestInfo with all required fields
  - [ ] Pass authenticated principal
  - [ ] Pass parsed request body data

- [ ] Handle authorization response
  - [ ] Check `allowed` bool
  - [ ] Return 403 Forbidden with reason if not allowed
  - [ ] Log authorization decisions (optional)
  - [ ] Handle errors gracefully

## Testing

- [ ] Unit tests pass
  - [ ] Run `go test ./internal/authorization -v`
  - [ ] Verify all tests pass
  - [ ] Check test coverage

- [ ] Write integration tests
  - [ ] Use `TestHelper` for easier testing
  - [ ] Create mock plainId responses
  - [ ] Test allow scenarios
  - [ ] Test deny scenarios
  - [ ] Test error scenarios

- [ ] Manual testing
  - [ ] Start plainId service
  - [ ] Start application
  - [ ] Test authorized requests (expect success)
  - [ ] Test unauthorized requests (expect 403)
  - [ ] Test with missing fields (expect error handling)
  - [ ] Test with invalid JSON paths (expect graceful failure)

## Documentation

- [ ] Review documentation files
  - [ ] Read `plainid-authorization.md` for technical details
  - [ ] Review `plainid-usage-guide.md` for integration steps
  - [ ] Check `plainid-config-example.yaml` for configuration patterns

- [ ] Document custom configuration
  - [ ] Document all endpoints with their rulesets
  - [ ] Document field mappings for each endpoint
  - [ ] Create examples for your specific use cases

- [ ] Add comments to code
  - [ ] Comment the authorization middleware
  - [ ] Document any custom request building logic
  - [ ] Add notes about special field extractions

## Deployment

- [ ] Prepare deployment
  - [ ] Ensure plainId service is accessible from deployment environment
  - [ ] Verify network/firewall rules allow connection to plainId
  - [ ] Update configuration management with plainId credentials

- [ ] Deploy code
  - [ ] Deploy application with plainId authorization
  - [ ] Verify configuration is loaded correctly
  - [ ] Confirm authorization checks are active

- [ ] Monitor in production
  - [ ] Check logs for authorization errors
  - [ ] Monitor plainId service availability
  - [ ] Track authorization decision patterns
  - [ ] Monitor performance impact

## Troubleshooting Checklist

If plainId checks are not working:

- [ ] Verify plainId service is running
  - [ ] Check plainId service health endpoint
  - [ ] Verify network connectivity to plainId

- [ ] Check configuration
  - [ ] Confirm `authorization.yaml` is loaded
  - [ ] Verify `enabled: true` in finegrain-check
  - [ ] Check validation-url is correct
  - [ ] Verify credentials are correct

- [ ] Check request matching
  - [ ] Verify request path matches a pattern in resource-map
  - [ ] Check if method is specified in pattern
  - [ ] Confirm pattern matching logic

- [ ] Check field extraction
  - [ ] Verify JSON paths in config match request body
  - [ ] Check for nested field access issues
  - [ ] Verify array extraction syntax is correct

- [ ] Check response handling
  - [ ] Verify plainId is returning valid JSON
  - [ ] Check response includes permit/deny/allow field
  - [ ] Monitor HTTP status codes from plainId

- [ ] Enable debug logging
  - [ ] Add detailed logging in middleware
  - [ ] Log request info before plainId call
  - [ ] Log plainId request being sent
  - [ ] Log plainId response received
  - [ ] Log final authorization decision

## Performance Considerations

- [ ] Monitor authorization latency
  - [ ] Track plainId service response times
  - [ ] Identify slow endpoints
  - [ ] Consider caching if needed

- [ ] Optimize field extraction
  - [ ] Use efficient JSON paths
  - [ ] Avoid unnecessary array processing
  - [ ] Consider field validation before extraction

- [ ] Load testing
  - [ ] Test with expected traffic volume
  - [ ] Monitor plainId service under load
  - [ ] Check for timeout issues
  - [ ] Verify graceful degradation

## Maintenance

- [ ] Keep documentation updated
  - [ ] Document new endpoints and rules
  - [ ] Update examples as patterns change
  - [ ] Keep plainId integration guide current

- [ ] Regular review
  - [ ] Review authorization logs monthly
  - [ ] Check for misconfigured rules
  - [ ] Verify all active endpoints have rules

- [ ] Update planning
  - [ ] Plan for plainId API version updates
  - [ ] Monitor plainId deprecation notices
  - [ ] Test compatibility with new versions

## Security Review

- [ ] Security best practices
  - [ ] Credentials are not hardcoded
  - [ ] Use environment variables for secrets
  - [ ] Credentials are rotated regularly
  - [ ] PlainId service connection is secure (HTTPS)

- [ ] Authorization review
  - [ ] All sensitive endpoints have rules
  - [ ] Rules are correctly configured
  - [ ] No unintended fail-open scenarios
  - [ ] Audit logging is enabled

- [ ] Testing coverage
  - [ ] Test with various request payloads
  - [ ] Test edge cases (empty fields, null values)
  - [ ] Test error scenarios
  - [ ] Test with invalid/malicious input

## Sign-off

- [ ] Development complete
  - [ ] All code implemented and tested
  - [ ] Documentation is complete
  - [ ] Code review completed

- [ ] QA testing complete
  - [ ] All test cases passed
  - [ ] No regressions detected
  - [ ] Performance acceptable

- [ ] Production ready
  - [ ] Configuration validated
  - [ ] Deployment plan confirmed
  - [ ] Rollback plan ready
  - [ ] Monitoring setup complete

---

## Reference Links

- **Technical Documentation**: `documentation/plainid-authorization.md`
- **Usage Guide**: `documentation/plainid-usage-guide.md`
- **Configuration Examples**: `documentation/plainid-config-example.yaml`
- **Implementation Summary**: `documentation/PLAINID_IMPLEMENTATION.md`
- **PlainId API Docs**: https://docs.plainid.io/apidocs/v5-permit-deny

## Contact & Support

- For technical questions about the implementation, see the documentation files
- For plainId service issues, contact plainId support
- For integration questions, refer to the usage guide

