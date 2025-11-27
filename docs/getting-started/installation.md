---
title: "Installation"
description: "Download and install PHPeek PM for your platform in minutes"
weight: 3
---

# Installation

PHPeek PM is distributed as a single static binary with zero dependencies, making installation straightforward.

## Download Pre-Built Binary

### Latest Release

Download the latest version from GitHub releases:

```bash
# Linux AMD64
wget https://github.com/gophpeek/phpeek-pm/releases/latest/download/phpeek-pm-linux-amd64
chmod +x phpeek-pm-linux-amd64
mv phpeek-pm-linux-amd64 /usr/local/bin/phpeek-pm

# Linux ARM64
wget https://github.com/gophpeek/phpeek-pm/releases/latest/download/phpeek-pm-linux-arm64
chmod +x phpeek-pm-linux-arm64
mv phpeek-pm-linux-arm64 /usr/local/bin/phpeek-pm

# macOS AMD64
wget https://github.com/gophpeek/phpeek-pm/releases/latest/download/phpeek-pm-darwin-amd64
chmod +x phpeek-pm-darwin-amd64
mv phpeek-pm-darwin-amd64 /usr/local/bin/phpeek-pm

# macOS ARM64 (Apple Silicon)
wget https://github.com/gophpeek/phpeek-pm/releases/latest/download/phpeek-pm-darwin-arm64
chmod +x phpeek-pm-darwin-arm64
mv phpeek-pm-darwin-arm64 /usr/local/bin/phpeek-pm
```

### Verify Installation

```bash
phpeek-pm --version
# Output: PHPeek Process Manager v1.0.0
```

## Docker Installation

### Using Official Image (Recommended)

```dockerfile
FROM gophpeek/phpeek-pm:latest AS phpeek

FROM php:8.3-fpm-alpine

# Copy phpeek-pm binary
COPY --from=phpeek /usr/local/bin/phpeek-pm /usr/local/bin/phpeek-pm

# Copy configuration
COPY phpeek-pm.yaml /etc/phpeek-pm/phpeek-pm.yaml

ENTRYPOINT ["/usr/local/bin/phpeek-pm"]
```

### Download in Dockerfile

```dockerfile
FROM php:8.3-fpm-alpine

# Install PHPeek PM
RUN wget -O /usr/local/bin/phpeek-pm \
    https://github.com/gophpeek/phpeek-pm/releases/latest/download/phpeek-pm-linux-amd64 \
    && chmod +x /usr/local/bin/phpeek-pm

# Copy configuration
COPY phpeek-pm.yaml /etc/phpeek-pm/phpeek-pm.yaml

ENTRYPOINT ["/usr/local/bin/phpeek-pm"]
```

## Build from Source

### Prerequisites

- Go 1.23 or later
- Git

### Clone and Build

```bash
# Clone repository
git clone https://github.com/gophpeek/phpeek-pm.git
cd phpeek-pm

# Build for current platform
make build

# Binary created at: build/phpeek-pm
./build/phpeek-pm --version

# Build for all platforms
make build-all

# Binaries created in build/ directory:
# - phpeek-pm-linux-amd64
# - phpeek-pm-linux-arm64
# - phpeek-pm-darwin-amd64
# - phpeek-pm-darwin-arm64
```

### Build Options

```bash
# Development build (includes debug symbols)
make dev

# Run tests
make test

# Clean build artifacts
make clean

# Install dependencies
make deps
```

## Configuration Setup

Create a basic configuration file:

```bash
# Create directory
sudo mkdir -p /etc/phpeek-pm

# Create minimal configuration
cat > /etc/phpeek-pm/phpeek-pm.yaml <<EOF
version: "1.0"

global:
  shutdown_timeout: 30
  log_level: info

processes:
  php-fpm:
    enabled: true
    command: ["php-fpm", "-F", "-R"]
    restart: always
EOF
```

## Verify Installation

Test your installation with a simple configuration:

```bash
# Run with explicit config path
phpeek-pm --config phpeek-pm.yaml

# Or use environment variable
PHPEEK_PM_CONFIG=phpeek-pm.yaml phpeek-pm

# Or use default location
# PHPeek PM looks for config in order:
# 1. PHPEEK_PM_CONFIG env var
# 2. /etc/phpeek-pm/phpeek-pm.yaml
# 3. ./phpeek-pm.yaml (current directory)
```

## Platform Support

| Platform | Architecture | Status |
|----------|--------------|--------|
| Linux | AMD64 | ✅ Full Support |
| Linux | ARM64 | ✅ Full Support |
| macOS | AMD64 | ✅ Full Support |
| macOS | ARM64 | ✅ Full Support |
| Windows | - | ❌ Not Supported |

## System Requirements

**Minimum**
- 512MB RAM
- 50MB disk space
- Linux kernel 3.10+ or macOS 10.15+

**Recommended**
- 1GB+ RAM (depends on managed processes)
- 100MB disk space
- Recent Linux kernel (5.x+) or macOS 12+

## Next Steps

Now that PHPeek PM is installed, proceed to:

- [Quick Start](quickstart) - Run your first multi-process setup
- [Docker Integration](docker-integration) - Use PHPeek PM as PID 1
- [Configuration](../configuration/overview) - Learn about configuration options
