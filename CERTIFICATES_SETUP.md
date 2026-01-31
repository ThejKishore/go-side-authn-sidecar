# CA Certificates Setup Guide

This guide explains how to use pre-downloaded CA certificates in your Docker image instead of installing them at build time using `apk add ca-certificates`.

## Benefits

- **Faster Docker builds**: No need to download certificates during each build
- **Deterministic builds**: Same certificates every time
- **Smaller layer bloat**: Reuse the same certificate layer
- **Offline builds**: Build Docker images without network access (once certs are downloaded)

## Setup Instructions

### Step 1: Run the Certificate Download Script

Execute the provided shell script to download CA certificates locally:

```bash
./scripts/download-certs.sh
```

This script will:
1. Create a `certs/` directory in your project root
2. Download Mozilla's CA certificate bundle
3. Set up the proper Alpine Linux directory structure (`certs/etc/ssl/certs/`)
4. Create the certificates ready for Docker COPY

**Output:**
```
Downloading CA certificates...
✓ CA bundle downloaded successfully to ./certs/ca-bundle.crt
  Size: 256K
✓ Certificates ready for Docker COPY
```

### Step 2: Build Your Docker Image

Build the Docker image as usual:

```bash
docker build -t reverse-proxy:latest .
```

The Dockerfile will now copy the pre-downloaded certificates instead of downloading them during build time.

### Step 3: (Optional) Update Certificates Regularly

Certificates can expire or need updating. Re-run the script periodically to keep them current:

```bash
./scripts/download-certs.sh
```

Then rebuild your Docker image with the updated certificates.

## Directory Structure

After running the script, your directory structure will look like:

```
reverseProxy/
├── scripts/
│   └── download-certs.sh
├── certs/
│   ├── ca-bundle.crt           # Mozilla CA bundle
│   └── etc/ssl/certs/
│       └── ca-certificates.crt # Copy for Alpine
├── Dockerfile
├── go.mod
└── ...
```

## Dockerfile Changes

### Before:
```dockerfile
FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=build /app/main .
```

### After:
```dockerfile
FROM alpine:latest
COPY certs/etc/ssl/certs /etc/ssl/certs/
COPY --from=build /app/main .
```

## Git Considerations

The `certs/` directory is fairly large (200-300KB). You have two options:

### Option A: Commit certificates to git
If you want reproducible builds across all environments:

```bash
git add certs/
git commit -m "Add CA certificates for Docker"
```

### Option B: Use `.gitignore` (Recommended for development)
If team members should run the script locally:

```bash
echo "certs/" >> .gitignore
```

Then document in your CI/CD pipeline:
```yaml
# In your CI/CD pipeline (GitHub Actions, GitLab CI, etc.)
- name: Download certificates
  run: ./scripts/download-certs.sh
```

## Verification

To verify the certificates are correctly set up in your Docker image:

```bash
docker run --rm reverse-proxy:latest ls -la /etc/ssl/certs/
```

You should see `ca-certificates.crt` in the output.

## Troubleshooting

### Problem: `curl: command not found` error in script
**Solution**: Install curl on your machine:
```bash
# macOS
brew install curl

# Linux (Ubuntu/Debian)
sudo apt-get install curl

# Linux (Fedora/RHEL)
sudo dnf install curl
```

### Problem: `COPY certs/... failed: stat certs: no such file or directory`
**Solution**: You forgot to run the script first:
```bash
./scripts/download-certs.sh
```

### Problem: Certificate validation errors in Docker container
**Solution**: Rebuild the image with updated certificates:
```bash
./scripts/download-certs.sh
docker build --no-cache -t reverse-proxy:latest .
```

## References

- [Mozilla CA Certificate Bundle](https://curl.se/ca/cacert.pem)
- [Alpine Linux CA Certificates](https://wiki.alpinelinux.org/wiki/Modifying_system)
- [Docker COPY Documentation](https://docs.docker.com/engine/reference/builder/#copy)

