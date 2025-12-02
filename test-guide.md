### Goal
Run the reverse-proxy locally, bring up Keycloak in Docker, obtain a JWT from Keycloak, and call the proxy endpoint with the token.

### Prerequisites
- Docker (and Docker Compose v2+ — you can use `docker compose ...`)
- Go 1.25+
- Ports used locally:
  - Keycloak: `http://localhost:8080`
  - Reverse proxy app: `http://localhost:3001`

### 1) Start Keycloak with the pre-configured realm
The repo includes a Keycloak compose file and a realm export that will be imported on startup.
- Compose file: `docker/keycloack-compose.yml`
- Realm file (auto-imported): `docker/keycloak/baeldung-keycloak-realm.json`

Start Keycloak:
```bash
docker compose -f docker/keycloack-compose.yml up -d
```

Wait until it’s healthy. You can check logs:
```bash
docker logs -f keycloak-lcl
```

Admin console is available at:
- URL: `http://localhost:8080`
- Admin credentials (from compose env): `admin` / `admin`

The imported realm is named `baeldung-keycloak`.

### 2) Confirm the realm and clients
Open the admin console and switch to realm `baeldung-keycloak` (top-left dropdown). The realm import includes several clients, among them:
- `baeldung-keycloak-confidential` (confidential; secret: `secret`; Authorization Code enabled; Direct Access Grants OFF by default)
- `admin-cli` (public; Direct Access Grants ON)

For simple CLI-based token retrieval without configuring redirects, it’s easiest to use Resource Owner Password Credentials (ROPC) with the `admin-cli` client. That requires a realm user with a known password.

### 3) Create a test user (if you don’t already have one)
The imported users (`brice`, `igor`) do not include plaintext passwords you can use. Create your own test user:
1. In the `baeldung-keycloak` realm, go to `Users` → `Add user`.
2. Set `Username`: `tester` (or any name), and click `Create`.
3. Open the new user → `Credentials` tab → `Set password`.
   - Enter a password (e.g., `tester123!`).
   - Set `Temporary` to `OFF` and save.

### 4) Obtain an access token (easy path: ROPC with admin-cli)
Use the `admin-cli` client for password grant. Execute:
```bash
curl -s -X POST "http://localhost:8080/realms/baeldung-keycloak/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d 'grant_type=password' \
  -d 'client_id=admin-cli' \
  -d 'username=tester' \
  -d 'password=tester123!' | jq -r .access_token
```
If you don’t have `jq`, omit the pipe and copy the `access_token` value manually.

Save the token into a shell variable for convenience:
```bash
TOKEN=$(curl -s \
  -X POST "http://localhost:8080/realms/baeldung-keycloak/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d 'grant_type=password' \
  -d 'client_id=admin-cli' \
  -d 'username=tester' \
  -d 'password=tester123!' | jq -r .access_token)
```

Notes:
- This obtains a user token signed by the `baeldung-keycloak` realm using RSA (`RS256`).
- The reverse proxy validates tokens against the JWKS at `http://localhost:8080/realms/baeldung-keycloak/protocol/openid-connect/certs`.
 - If your shell complains about missing quotes when running the `curl` command, it’s usually due to `!` history expansion in Bash. Using single quotes for `-d` arguments (as shown) avoids that issue. Alternatively, escape the exclamation mark in the password (e.g., `tester123\!`) or run `set +H` in Bash to disable history expansion for the session.

Alternative ways to get a token (optional):
- Use `baeldung-keycloak-confidential`:
  - In Clients → `baeldung-keycloak-confidential`, you can enable `Direct Access Grants` to allow ROPC for that client, or enable `Service accounts` and then use `client_credentials` grant with `client_secret=secret`. Client-credentials tokens won’t have user claims, but your proxy only verifies signature/validity, so it still works.
- Use Authorization Code flow via browser with `baeldung-keycloak-confidential` if you prefer a full OIDC redirect flow (requires a running app at the configured redirect URI `http://localhost:8081/login/oauth2/code/keycloak`).

### 5) Run the reverse-proxy locally
The app pulls JWKS from Keycloak at startup. In `cmd/reverse-proxy/main.go`:
```
jwksURL := "http://localhost:8080/realms/baeldung-keycloak/protocol/openid-connect/certs"
```
Start the app:
```bash
go run ./cmd/reverse-proxy
```
By default it listens on `:3001`.

### 6) Call the proxy endpoint with the token
The proxy forwards to `https://httpbin.org` while validating your JWT.

Example call:
```bash
curl -i \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:3001/anything"
```
Expected:
- HTTP 200 OK
- A JSON response from httpbin.org echoing your request (the proxy targets `https://httpbin.org` + original path).
- If the token is missing/invalid/expired, you’ll get `401 Unauthorized` from the proxy.

### 7) Troubleshooting
- Keycloak not reachable:
  - Ensure `docker compose -f docker/keycloack-compose.yml ps` shows `keycloak-lcl` up.
  - Try `curl http://localhost:8080/realms/baeldung-keycloak/.well-known/openid-configuration`.
- Token request fails:
  - For ROPC with `admin-cli`, verify the user exists and the password is non-temporary.
  - Confirm you’re posting to the right realm: `/realms/baeldung-keycloak/...`.
- Proxy returns 401:
  - Check app logs for JWKS fetch errors on startup.
  - Verify system time (JWT validation fails if your clock is far off).
  - Decode the token at `jwt.io` to confirm `alg` is `RS256` and `kid` exists in JWKS.
- Port conflicts:
  - Keycloak uses 8080, proxy uses 3001. Stop anything else on those ports.

### 8) Cleanup
Stop and remove Keycloak:
```bash
docker compose -f docker/keycloack-compose.yml down -v
```

### Appendix: Helpful URLs
- Realm OpenID config: `http://localhost:8080/realms/baeldung-keycloak/.well-known/openid-configuration`
- JWKS: `http://localhost:8080/realms/baeldung-keycloak/protocol/openid-connect/certs`
- Token endpoint: `http://localhost:8080/realms/baeldung-keycloak/protocol/openid-connect/token`
- Admin console: `http://localhost:8080/admin/`

### Why this works with the current code
- The reverse proxy reads `Authorization: Bearer <token>`, extracts the `kid`, fetches the matching public key from JWKS (cached in-memory), and verifies the token using RS256.
- On success, it stores a `Principal` in `fiber.Ctx.Locals` and proxies the request to `https://httpbin.org` preserving your path.
