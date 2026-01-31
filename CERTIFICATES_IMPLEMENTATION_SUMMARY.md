# CA Certificates Setup - Implementation Summary

## ✅ What Has Been Done

### 1. **Created Shell Script** (`scripts/download-certs.sh`)
   - Downloads Mozilla's CA certificate bundle using `curl`
   - Creates proper Alpine Linux directory structure
   - Sets up certificates in `certs/etc/ssl/certs/` for Docker COPY
   - Includes helpful output and verification messages

### 2. **Updated Dockerfile**
   - Replaced `RUN apk --no-cache add ca-certificates` with local certificate copying
   - Now uses: `COPY certs/etc/ssl/certs /etc/ssl/certs/`
   - Significantly faster builds (no network I/O during build)
   - More deterministic builds (same certificates every time)

### 3. **Downloaded Certificates**
   - Executed the script: `bash scripts/download-certs.sh`
   - Downloaded 220KB CA certificate bundle
   - Created directory structure:
     ```
     certs/
     ├── ca-bundle.crt              (220KB - Mozilla CA bundle)
     └── etc/ssl/certs/
         └── ca-certificates.crt    (Same bundle for Alpine)
     ```

### 4. **Created Documentation**
   - `CERTIFICATES_SETUP.md` - Complete setup guide with troubleshooting
   - `documentation/CICD_CERTIFICATES_SETUP.md` - CI/CD pipeline examples (GitHub Actions, GitLab CI, Jenkins)

## 📝 How to Use

### Quick Start
```bash
# 1. The script has already been run, but to update certificates later:
bash scripts/download-certs.sh

# 2. Build Docker image (certificates will be copied from certs/ directory)
docker build -t reverse-proxy:latest .

# 3. Verify certificates in container
docker run --rm reverse-proxy:latest ls -la /etc/ssl/certs/
```

### Git Management

**Option A: Commit certificates (reproducible builds)**
```bash
git add certs/
git commit -m "Add CA certificates for Docker builds"
```

**Option B: Ignore certificates (each builder downloads)**
```bash
echo "certs/" >> .gitignore
# Document in CI/CD pipeline to run: bash scripts/download-certs.sh
```

## 🎯 Benefits

| Aspect | Before | After |
|--------|--------|-------|
| **Build Time** | Slower (downloads certs) | Faster (uses pre-downloaded) |
| **Build Determinism** | Variable (depends on availability) | Deterministic (same certs) |
| **Offline Builds** | Requires network | Works offline (if certs present) |
| **Dockerfile Layers** | Smaller layers, more builds | Reusable cert layers |
| **Image Size** | ~220KB (download + cache) | ~220KB (static COPY) |

## 📂 Files Created/Modified

### Created Files:
- ✅ `scripts/download-certs.sh` - Certificate download script
- ✅ `CERTIFICATES_SETUP.md` - User guide
- ✅ `documentation/CICD_CERTIFICATES_SETUP.md` - CI/CD examples
- ✅ `certs/ca-bundle.crt` - Downloaded CA bundle
- ✅ `certs/etc/ssl/certs/ca-certificates.crt` - Alpine-compatible copy

### Modified Files:
- ✅ `Dockerfile` - Changed from `RUN apk --no-cache add ca-certificates` to `COPY certs/etc/ssl/certs /etc/ssl/certs/`

## 🔄 Updating Certificates

Certificates should be updated periodically (typically annually or when issues arise):

```bash
# Update certificates
bash scripts/download-certs.sh

# Rebuild image (recommended to use --no-cache to ensure fresh build)
docker build --no-cache -t reverse-proxy:latest .

# Commit changes if using git
git add certs/
git commit -m "Update CA certificates"
```

## 🐳 Docker Build Flow

```
docker build
    ↓
FROM golang:1.23-alpine
    ↓
COPY go.mod go.sum ./ + RUN go mod tidy
    ↓
COPY . . + RUN go build -o main
    ↓
FROM alpine:latest
    ↓
COPY certs/etc/ssl/certs /etc/ssl/certs/  ← Uses pre-downloaded certs
    ↓
COPY --from=build /app/main .
    ↓
Final image ready with HTTPS support
```

## ✅ Next Steps

1. **Optional: Commit to Git**
   ```bash
   git add scripts/download-certs.sh certs/ CERTIFICATES_SETUP.md documentation/CICD_CERTIFICATES_SETUP.md
   git commit -m "Add local CA certificates for faster Docker builds"
   ```

2. **Optional: Setup CI/CD**
   - Choose an example from `documentation/CICD_CERTIFICATES_SETUP.md`
   - Add to your CI/CD pipeline to download certificates before build

3. **Test the Build**
   ```bash
   docker build -t reverse-proxy:latest .
   docker run --rm reverse-proxy:latest ls /etc/ssl/certs/
   ```

4. **Documentation**
   - Team should refer to `CERTIFICATES_SETUP.md` for certificate updates
   - CI/CD engineers refer to `documentation/CICD_CERTIFICATES_SETUP.md`

## 🆘 Troubleshooting

### Build fails with "COPY certs/... no such file"
→ Run: `bash scripts/download-certs.sh`

### Certificates appear invalid in container
→ Rebuild with `--no-cache` to ensure fresh fetch:
```bash
bash scripts/download-certs.sh
docker build --no-cache -t reverse-proxy:latest .
```

### Old certificates causing issues
→ Update and rebuild:
```bash
bash scripts/download-certs.sh
docker build --no-cache -t reverse-proxy:latest .
```

## 📚 References

- Script downloads from: https://curl.se/ca/cacert.pem (Mozilla's trusted root store)
- Alpine SSL paths: `/etc/ssl/certs/ca-certificates.crt`
- Docker COPY documentation: https://docs.docker.com/engine/reference/builder/#copy

