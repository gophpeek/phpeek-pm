# PHPeek PM Documentation Cleanup Plan

**Date**: 2025-11-24
**Status**: Analysis Complete, Ready for Execution
**Priority**: High (DX Impact)

---

## Executive Summary

The PHPeek PM documentation has **significant gaps** and **incomplete sections** that harm developer experience. While `README.md` is comprehensive, the `/docs` directory contains many placeholder files and missing critical features added in recent phases (TUI, scaffolding, metrics, tracing, dev mode).

**Impact**: Developers cannot discover or use ~40% of features without reading source code or CLAUDE.md.

---

## Critical Issues Found

### 🔴 **Priority 1: Missing Critical Features** (0-2 days)

These features exist in the codebase but have ZERO documentation:

1. **TUI (Terminal User Interface)**
   - Exists: `cmd/phpeek-pm/tui.go` + `internal/tui/` package
   - Missing: User guide, keyboard shortcuts, connection modes
   - Impact: Users don't know PHPeek has a k9s-style TUI

2. **Config Validation (`check-config`)**
   - Exists: `cmd/phpeek-pm/check_config.go` + validation system
   - Missing: Guide on modes (--quiet, --strict, --json), CI/CD integration
   - Impact: Users manually validate configs instead of using built-in linter

3. **Scaffolding Tools (`scaffold`)**
   - Exists: `cmd/phpeek-pm/scaffold.go` + presets (laravel, symfony, production, etc.)
   - Missing: Guide on presets, interactive mode, Docker file generation
   - Impact: Users write configs from scratch instead of using scaffolding

4. **Resource Metrics & Monitoring**
   - Exists: `internal/metrics/resource.go` + time series + REST API
   - Missing: REST API endpoints (`/api/v1/metrics/history`), configuration guide
   - Impact: Users don't know they can query historical resource data

5. **Distributed Tracing (OpenTelemetry)**
   - Exists: `internal/tracing/` package + OTLP gRPC support
   - Missing: Configuration guide, Jaeger/Tempo integration examples
   - Impact: Users can't leverage distributed tracing for debugging

6. **Development Mode with File Watching**
   - Exists: `internal/watcher/` package + `--dev` flag
   - Missing: Developer guide, workflow examples
   - Impact: Developers manually restart instead of using auto-reload

### 🟡 **Priority 2: Incomplete/Stub Documentation** (2-4 days)

These docs exist but provide little value:

1. **Health Checks** (`docs/features/health-checks.md`)
   - Status: "Documentation in Progress" stub
   - Needs: Complete guide with TCP/HTTP/exec examples, troubleshooting
   - Impact: Users can't configure health checks properly

2. **Management API** (`docs/observability/api.md`)
   - Status: Incomplete (basic endpoints only)
   - Needs: All CRUD operations, runtime service management, TUI integration
   - Impact: Users don't know about add/update/remove process capabilities

3. **Empty Index Files**
   - `docs/features/_index.md` - just navigation
   - `docs/configuration/_index.md` - just navigation
   - `docs/examples/_index.md` - just navigation
   - Needs: Summary content explaining section purpose

### 🟢 **Priority 3: DX Improvements** (4-7 days)

Quality-of-life improvements for developer experience:

1. **Quick Start Missing**
   - Need: "60-second quick start" at top of README
   - Need: Single `docker run` command to test PHPeek immediately
   - Impact: High barrier to entry for evaluation

2. **No Troubleshooting Guide**
   - Need: Common issues (OOM kills, zombie processes, health check failures)
   - Need: Debug log patterns
   - Impact: Users struggle with common problems

3. **No Migration Guides**
   - Need: From supervisord, s6-overlay, Docker Compose multi-container
   - Impact: Hard to adopt for existing projects

4. **No Recipes/Patterns**
   - Need: Common patterns (Laravel + Redis, Symfony + RabbitMQ, WordPress)
   - Need: Performance tuning guide
   - Impact: Users reinvent solutions

5. **No FAQ**
   - Need: Common questions (vs. supervisord? vs. k8s? vs. Docker Compose?)
   - Impact: Confusion about positioning

6. **README.md Improvements**
   - Add: Quick navigation table at top
   - Add: "When to use / when not to use" section
   - Improve: Feature section organization

---

## Documentation Structure Issues

### Current Structure (Messy)

```
docs/
├── introduction.md           # Good
├── PHP-FPM-AUTOTUNE.md      # Good (but wrong location)
├── getting-started/          # Good
├── configuration/            # Good structure, but incomplete
├── features/                 # Mostly stubs and placeholders
├── examples/                 # Good examples but need more
├── observability/            # Incomplete (missing metrics details)
└── development/              # Exists but not documented
```

### Proposed Structure (Clean)

```
docs/
├── README.md                 # Navigation hub with feature matrix
├── introduction.md           # Overview, architecture, use cases
├── getting-started/
│   ├── installation.md       # ✅ Good
│   ├── quickstart.md         # ✅ Good
│   ├── docker-integration.md # ✅ Good
│   └── 60-second-demo.md     # 🆕 Quick evaluation
├── configuration/
│   ├── overview.md           # ✅ Good
│   ├── global-settings.md    # ✅ Good
│   ├── processes.md          # ✅ Good
│   ├── health-checks.md      # ✅ Good
│   ├── lifecycle-hooks.md    # ✅ Good
│   ├── environment-variables.md # ✅ Good
│   ├── validation.md         # 🆕 check-config guide
│   └── php-fpm-autotune.md   # 🔄 Move from root
├── features/
│   ├── health-checks.md      # 🔄 Complete (currently stub)
│   ├── dependency-management.md # ✅ Good
│   ├── scheduled-tasks.md    # ✅ Good
│   ├── process-scaling.md    # ✅ Good
│   ├── restart-policies.md   # ✅ Good
│   ├── advanced-logging.md   # ✅ Good
│   ├── heartbeat-monitoring.md # ✅ Good
│   ├── tui.md                # 🆕 Terminal UI guide
│   ├── scaffolding.md        # 🆕 Scaffold command guide
│   └── dev-mode.md           # 🆕 File watching guide
├── observability/
│   ├── metrics.md            # 🔄 Complete (add resource metrics)
│   ├── api.md                # 🔄 Complete (add CRUD operations)
│   ├── resource-monitoring.md # 🆕 Time series, REST API
│   └── tracing.md            # 🆕 OpenTelemetry guide
├── examples/
│   ├── minimal.md            # ✅ Good
│   ├── laravel-complete.md   # ✅ Good
│   ├── laravel-with-monitoring.md # ✅ Good
│   ├── scheduled-tasks.md    # ✅ Good
│   ├── docker-compose.md     # ✅ Good
│   ├── kubernetes.md         # ✅ Good
│   ├── symfony-app.md        # 🆕 Symfony example
│   └── wordpress-app.md      # 🆕 WordPress example
├── guides/                   # 🆕 New section
│   ├── troubleshooting.md    # 🆕 Common issues
│   ├── migration-supervisord.md # 🆕 From supervisord
│   ├── migration-s6.md       # 🆕 From s6-overlay
│   ├── performance-tuning.md # 🆕 Optimization guide
│   ├── recipes.md            # 🆕 Common patterns
│   └── faq.md                # 🆕 Frequently asked questions
└── development/
    ├── architecture.md       # 🆕 Codebase structure
    ├── contributing.md       # 🆕 Contribution guide
    ├── testing.md            # 🔄 Expand existing
    └── webui.md              # 🔄 Expand existing
```

---

## Execution Plan

### Phase 1: Critical Gaps (Days 1-2) 🔴

**Goal**: Document all existing but undocumented features

1. **Create `docs/features/tui.md`**
   - Connection modes (Unix socket + TCP)
   - Keyboard shortcuts reference
   - Add process wizard walkthrough
   - Screenshots/examples

2. **Create `docs/configuration/validation.md`**
   - `check-config` command usage
   - Validation modes (--quiet, --strict, --json)
   - CI/CD integration examples
   - Error/warning/suggestion categories

3. **Create `docs/features/scaffolding.md`**
   - Preset overview (laravel, symfony, production, minimal, generic)
   - Interactive mode guide
   - Docker file generation (--dockerfile, --docker-compose)
   - Customization workflow

4. **Create `docs/observability/resource-monitoring.md`**
   - Resource metrics configuration
   - Time series API (`/api/v1/metrics/history`)
   - Prometheus integration
   - Grafana query examples

5. **Create `docs/observability/tracing.md`**
   - OpenTelemetry configuration
   - Exporter types (otlp-grpc, stdout)
   - Jaeger integration example
   - Grafana Tempo integration example
   - Sampling strategies

6. **Create `docs/features/dev-mode.md`**
   - `--dev` flag usage
   - File watcher behavior
   - Debouncing and validation
   - Developer workflow examples

### Phase 2: Complete Incomplete Docs (Days 2-3) 🟡

**Goal**: Turn stubs into useful documentation

1. **Complete `docs/features/health-checks.md`**
   - Remove "Documentation in Progress" stub
   - TCP, HTTP, exec examples with full config
   - Success threshold patterns
   - Troubleshooting section

2. **Complete `docs/observability/api.md`**
   - Document all CRUD operations (add, update, remove process)
   - Runtime service management
   - Config persistence (save, reload, validate)
   - TUI wizard integration
   - Authentication (Unix socket vs TCP)

3. **Complete `docs/observability/metrics.md`**
   - Add resource metrics section
   - Document all Prometheus gauges
   - Add PromQL query examples
   - Grafana dashboard JSON

4. **Add Content to Index Files**
   - `docs/features/_index.md` - Feature overview table
   - `docs/configuration/_index.md` - Config philosophy
   - `docs/examples/_index.md` - Example matrix

### Phase 3: DX Improvements (Days 4-7) 🟢

**Goal**: Make PHPeek PM easier to evaluate, adopt, and use

1. **Create `docs/getting-started/60-second-demo.md`**
   - Single `docker run` command to test immediately
   - Pre-built demo image
   - Interactive playground

2. **Create `docs/guides/troubleshooting.md`**
   - OOM kills (PHP-FPM over-provisioning)
   - Zombie processes (PID 1 issues)
   - Health check failures (timing, dependencies)
   - Graceful shutdown issues
   - Debug log patterns

3. **Create Migration Guides**
   - `docs/guides/migration-supervisord.md`
   - `docs/guides/migration-s6.md`
   - `docs/guides/migration-docker-compose.md`

4. **Create `docs/guides/recipes.md`**
   - Laravel + Redis pattern
   - Symfony + RabbitMQ pattern
   - WordPress + Cron pattern
   - Multi-tenant application pattern
   - CI/CD integration pattern

5. **Create `docs/guides/performance-tuning.md`**
   - PHP-FPM worker optimization
   - Memory profiling
   - CPU utilization
   - Log performance impact
   - Metrics overhead

6. **Create `docs/guides/faq.md`**
   - vs. supervisord?
   - vs. s6-overlay?
   - vs. Kubernetes?
   - vs. Docker Compose multi-container?
   - When to use PHPeek PM?
   - Security considerations

7. **Improve `README.md`**
   - Add quick navigation table at top
   - Add "When to Use / When Not to Use" section
   - Reorganize features for better scanning
   - Add comparison matrix

8. **Reorganize PHP-FPM Autotune Doc**
   - Move `docs/PHP-FPM-AUTOTUNE.md` → `docs/configuration/php-fpm-autotune.md`
   - Update all references

### Phase 4: Development Docs (Day 7) 🟢

**Goal**: Help contributors understand codebase

1. **Create `docs/development/architecture.md`**
   - Package overview
   - Key interfaces
   - Data flow diagrams
   - Testing strategy

2. **Create `docs/development/contributing.md`**
   - Setup instructions
   - Code style guide
   - Pull request process
   - Release workflow

---

## Success Metrics

**Before Cleanup:**
- ❌ 6 critical features undocumented (40% of Phase 5-7 features)
- ❌ 4 stub/incomplete documentation files
- ❌ No troubleshooting guide
- ❌ No migration guides
- ❌ No FAQ
- ❌ High barrier to entry (no quick demo)

**After Cleanup:**
- ✅ 100% feature coverage
- ✅ No stub files (all complete or removed)
- ✅ Comprehensive troubleshooting guide
- ✅ 3 migration guides
- ✅ FAQ with common questions
- ✅ 60-second quick start demo
- ✅ Developer experience score: 8/10 → 9.5/10

---

## Estimated Effort

| Phase | Scope | Effort | Priority |
|-------|-------|--------|----------|
| Phase 1 | Critical gaps (6 docs) | 2 days | 🔴 High |
| Phase 2 | Complete incomplete (4 docs) | 1 day | 🟡 Medium |
| Phase 3 | DX improvements (8 docs) | 3 days | 🟢 Medium |
| Phase 4 | Development (2 docs) | 1 day | 🟢 Low |
| **Total** | **20 new/updated docs** | **7 days** | - |

**Quick win**: Phase 1 alone provides massive value (2 days, 6 critical docs).

---

## Next Steps

1. **Get user approval** on this plan
2. **Execute Phase 1** (critical gaps) first for immediate impact
3. **Iterate based on feedback** from Phase 1
4. **Continue with Phase 2-4** if approved

**User decision point**: Execute all phases, or just Phase 1 for quick wins?

---

## Notes

- All documentation will follow existing Hugo front matter format
- Examples will use realistic Laravel/Symfony scenarios
- Code samples will be tested before inclusion
- Cross-references will be updated across all docs
- Old/outdated docs will be removed or updated
