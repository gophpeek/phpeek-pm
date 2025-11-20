---
title: "Getting Started"
description: "Install PHPeek PM and run your first multi-process container in minutes"
weight: 2
---

# Getting Started

Get PHPeek PM up and running in your Docker containers quickly.

## What You'll Learn

This section covers everything you need to get started:

- **[Installation](installation)** - Download and install PHPeek PM binaries
- **[Quick Start](quickstart)** - 5-minute tutorial with working examples
- **[Docker Integration](docker-integration)** - Use PHPeek PM as PID 1 in containers

## Prerequisites

Before you begin, ensure you have:

- Docker or container runtime
- Basic understanding of YAML configuration
- PHP application to manage (optional for testing)

## Typical Setup Flow

```
1. Download PHPeek PM binary
   ↓
2. Create phpeek-pm.yaml configuration
   ↓
3. Build Docker image with PHPeek PM
   ↓
4. Run container with PHPeek PM as ENTRYPOINT
   ↓
5. Monitor via metrics and API
```

## Quick Links

- [Binary Downloads](https://github.com/gophpeek/phpeek-pm/releases)
- [Example Configurations](../examples/minimal)
- [Configuration Reference](../configuration/overview)

Start with [Installation](installation) to get the binary, then move to [Quick Start](quickstart) for a hands-on tutorial.
