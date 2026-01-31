# CI/CD Example: GitHub Actions

This example shows how to use the certificate download script in your CI/CD pipeline.

## GitHub Actions Workflow

Create `.github/workflows/docker-build.yml`:

```yaml
name: Build Docker Image

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  build:
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v3

    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v2

    - name: Download CA certificates
      run: |
        bash scripts/download-certs.sh

    - name: Build Docker image
      run: |
        docker build -t reverse-proxy:latest .

    - name: Test image
      run: |
        docker run --rm reverse-proxy:latest /main --help || true
```

## GitLab CI/CD

Create `.gitlab-ci.yml`:

```yaml
stages:
  - build

build_docker:
  stage: build
  image: docker:latest
  services:
    - docker:dind
  before_script:
    - apk add --no-cache bash curl
    - bash scripts/download-certs.sh
  script:
    - docker build -t reverse-proxy:latest .
    - docker run --rm reverse-proxy:latest /main --help || true
  only:
    - main
    - develop
```

## Jenkins Pipeline

Create `Jenkinsfile`:

```groovy
pipeline {
    agent any

    stages {
        stage('Prepare') {
            steps {
                sh 'bash scripts/download-certs.sh'
            }
        }
        
        stage('Build') {
            steps {
                sh 'docker build -t reverse-proxy:latest .'
            }
        }
        
        stage('Test') {
            steps {
                sh 'docker run --rm reverse-proxy:latest /main --help || true'
            }
        }
    }
}
```

## Docker Compose with Script

If you're using Docker Compose, you can automate the setup:

```bash
#!/bin/bash
# prepare-build.sh

set -e

echo "Preparing Docker build..."
bash scripts/download-certs.sh
echo "✓ Certificates downloaded"

docker-compose build reverse-proxy
echo "✓ Docker image built"
```

Then run: `bash prepare-build.sh`

