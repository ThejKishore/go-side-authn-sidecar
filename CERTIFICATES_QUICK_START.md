# CA Certificates - Quick Reference

## TL;DR - What Changed

**Before:**
```dockerfile
RUN apk --no-cache add ca-certificates
```

**After:**
```bash
# 1. Download certs once:
bash scripts/download-certs.sh

# 2. Dockerfile now uses pre-downloaded certs:
COPY certs/etc/ssl/certs /etc/ssl/certs/
```

## One-Time Setup

```bash
bash scripts/download-certs.sh
```

Creates:
- `certs/ca-bundle.crt` - Downloaded from Mozilla
- `certs/etc/ssl/certs/ca-certificates.crt` - Alpine compatible

## Build & Test

```bash
# Build
docker build -t reverse-proxy:latest .

# Verify certs
docker run --rm reverse-proxy:latest ls /etc/ssl/certs/
```

## Keep Certificates Current

```bash
# Update (run occasionally, e.g., annually)
bash scripts/download-certs.sh
docker build --no-cache -t reverse-proxy:latest .
```

## Files Overview

| File | Purpose |
|------|---------|
| `scripts/download-certs.sh` | Downloads certificates |
| `certs/` | Directory for downloaded certs |
| `Dockerfile` | Updated to use local certs |
| `CERTIFICATES_SETUP.md` | Full guide |
| `CERTIFICATES_IMPLEMENTATION_SUMMARY.md` | This implementation |
| `documentation/CICD_CERTIFICATES_SETUP.md` | CI/CD examples |

## Key Benefits

✅ Faster builds (no certificate downloads during build)  
✅ Deterministic builds (same certs every time)  
✅ Offline builds possible (after first download)  
✅ Smaller image layers  

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `COPY certs/ failed` | Run `bash scripts/download-certs.sh` |
| Old certificates | Run script again + rebuild with `--no-cache` |
| Script not found | Check you're in project root: `cd /Users/thejkaruneegar/GolandProjects/reverseProxy` |

## Git Recommendations

```bash
# Option 1: Commit certs (reproducible)
git add certs/ scripts/download-certs.sh
git commit -m "Add local CA certificates"

# Option 2: Ignore certs (let CI/CD download)
echo "certs/" >> .gitignore
# Then ensure CI/CD runs: bash scripts/download-certs.sh
```

