# Test Coverage Analysis

_Generated 2026-03-13_

## Coverage Summary

| Package | Coverage | Notes |
|---------|----------|-------|
| `internal/config` | **96.8%** | Excellent |
| `internal/download` | **77.3%** | Good |
| `internal/mirror` | **75.1%** | Good (1 flaky test: `TestSpeedTestWithErrors`) |
| `internal/provider` | **69.2%** | Moderate |
| `internal/provider/registry` | **60.1%** | Moderate |
| `internal/provider/containerimages` | **58.4%** | Needs improvement |
| `internal/ocp` | **57.3%** | Needs improvement |
| `internal/safety` | **52.8%** | Needs improvement |
| `internal/provider/ocp` | **48.3%** | Low |
| `internal/engine` | N/A | Could not measure (network dep) |
| `internal/server` | N/A | Could not measure (network dep) |
| `internal/store` | N/A | Could not measure (network dep) |
| `internal/jobsvc` | N/A | Could not measure (network dep) |
| `cmd/airgap` | N/A | Could not measure (network dep) |

**Test-to-source file ratio:** 34 test files / 57 source files = **59.6%**

---

## Files With No Test Coverage

### Critical Priority (Core Business Logic)

#### 1. `internal/server/handlers.go` — 1,147 lines, 20+ handlers
The single largest untested file. Contains the primary HTTP handlers for the
dashboard, provider detail views, sync triggering, validation, retry logic,
and progress streaming (SSE). This is the main user-facing surface area.

**What to test:**
- `handleDashboard` — renders provider summary
- `handleAPISync` / `handleAPISyncRetry` — sync trigger and retry flows
- `handleSyncProgress` — SSE progress streaming
- `handleAPIValidate` — validation triggering
- Error responses for missing/invalid providers
- HTML vs JSON content negotiation

#### 2. `internal/engine/import.go` — 364 lines
Handles importing transfer archives — archive extraction, manifest
verification, and file placement. The counterpart to `export.go` which *is*
well-tested. Without import tests, the export→import round-trip is only
partially validated.

**What to test:**
- Successful import of a valid archive
- Manifest mismatch / missing manifest detection
- Corrupt archive handling
- Path traversal protection during extraction
- Partial import recovery

#### 3. `internal/engine/progress.go` — 325 lines
Thread-safe progress tracking used by all sync operations and streamed to the
UI via SSE. Contains atomic counters, phase transitions, and snapshot logic.

**What to test:**
- Concurrent `UpdateFileProgress` / `FileCompleted` / `FileFailed` calls
- Phase transitions (`SetPhase`)
- Snapshot accuracy under concurrent mutations
- `Wait()` blocking/unblocking behavior

#### 4. `internal/server/server.go` — 234 lines
Server initialization, route registration, template parsing, and graceful
shutdown. Untested initialization could mask misconfigured routes.

**What to test:**
- Route registration (all expected paths are mounted)
- Graceful shutdown behavior
- Template parse errors are surfaced

### High Priority

#### 5. `internal/server/transfer_handlers.go` — 197 lines
Export/import operations exposed over HTTP API.

#### 6. `internal/server/ocp_handlers.go` — 267 lines
OpenShift artifact browsing and download handlers.

#### 7. `internal/jobsvc/service.go` — 197 lines
Job execution logic (`ExecuteJob`), provider name normalization and
validation, and persistence of validation reports. Only `cron_test.go` exists
for the scheduling side; the execution side is untested.

#### 8. `internal/safety/http.go` — 78 lines
HTTP safety utilities (`ValidateHTTPURL`, `IsLoopbackHost`,
`ReadAllWithLimit`). The companion `path.go` has tests but this file does
not. Security-sensitive code should always have tests.

### Medium Priority

#### 9. CLI commands (7 files, ~1,150 lines total)
`root.go`, `sync.go`, `serve.go`, `export.go`, `importcmd.go`,
`validate.go`, `status.go` — None of these have tests. Only `config_cmd.go`,
`jobs.go`, and `providers.go` are tested among the CLI commands.

#### 10. `internal/store/migrations.go` — 228 lines
Database schema migrations (7 versions). A migration regression could
silently break the database on upgrade.

#### 11. `internal/server/templates.go` — 61 lines
Template helper functions (`formatBytes`, `formatDuration`, etc.).

---

## Existing Test Quality Assessment

### Strengths
- **download/client_test.go** (592 lines, 13 tests) — Excellent. Tests retries,
  resume via Range headers, checksum validation, timeouts, and all HTTP error
  codes using `httptest`.
- **engine/sync_test.go** (1,123 lines, 21 tests) — Excellent. Custom mock
  providers, dry-run mode, partial failure scenarios, multi-provider
  orchestration, and context cancellation.
- **engine/export_test.go** (603 lines, 12 tests) — Excellent. Security-focused
  with tests for unsafe tar entries, symlinks, and path traversal. Good
  round-trip validation.
- **store/sqlite_test.go** (1,989 lines, 59 tests) — Very thorough CRUD
  coverage, though all in one file and no table-driven patterns.
- **config/config_test.go** (592 lines, 16 tests) — Good use of table-driven
  tests for `ProviderEnabled` and `ProviderDataDir`.

### Weaknesses
- **Table-driven tests underused** — Only `config_test.go` and `export_test.go`
  use them. Store CRUD tests and handler tests would benefit significantly.
- **Concurrency testing limited** — Only `pool_test.go` verifies concurrent
  behavior. `progress.go` (thread-safe by design) has zero concurrency tests.
- **No benchmarks** — Only `provider_test.go` includes benchmarks. Performance-
  sensitive paths (download pool, sync engine) have none.
- **Flaky test** — `TestSpeedTestWithErrors` in `mirror` package is failing.
- **Server handler tests are shallow** — `mirror_handlers_test.go` (142 lines)
  and `provider_config_handlers_test.go` (196 lines) cover basic CRUD but miss
  malformed input, concurrent requests, and error edge cases.

---

## Recommended Improvements

### 1. Add `internal/server/handlers_test.go`
**Impact: High | Effort: High**

This is the single biggest coverage gap. Start with the most critical handlers:

```go
func TestHandleDashboard(t *testing.T)        // verify HTML rendering with various provider states
func TestHandleAPISync(t *testing.T)           // verify sync trigger, already-running rejection
func TestHandleAPISyncRetry(t *testing.T)      // verify retry with failed files
func TestHandleSyncProgress(t *testing.T)      // verify SSE stream format
func TestHandleAPIValidate(t *testing.T)       // verify validation trigger
func TestHandleProviderDetail(t *testing.T)    // verify provider detail rendering
```

Use the existing pattern from `provider_config_handlers_test.go` which creates
a test server with a real SQLite store.

### 2. Add `internal/engine/import_test.go`
**Impact: High | Effort: Medium**

Complete the export→import round-trip. The export tests already create valid
archives; extend them:

```go
func TestImportValidArchive(t *testing.T)      // happy path
func TestImportCorruptArchive(t *testing.T)    // truncated/invalid zstd
func TestImportMissingManifest(t *testing.T)   // archive without manifest.json
func TestImportPathTraversal(t *testing.T)     // malicious tar entries
func TestImportIdempotent(t *testing.T)        // re-import same archive
```

### 3. Add `internal/engine/progress_test.go`
**Impact: Medium | Effort: Low**

Thread-safe code needs concurrency tests:

```go
func TestProgressConcurrentUpdates(t *testing.T)  // goroutines calling Update/Complete/Fail
func TestProgressSnapshot(t *testing.T)            // snapshot accuracy
func TestProgressPhaseTransitions(t *testing.T)    // phase ordering
func TestProgressWait(t *testing.T)                // blocking until done
```

### 4. Add `internal/safety/http_test.go`
**Impact: Medium | Effort: Low**

Security code must be tested:

```go
func TestValidateHTTPURL(t *testing.T)         // table-driven: valid URLs, non-HTTP schemes, empty
func TestIsLoopbackHost(t *testing.T)           // 127.0.0.1, localhost, ::1, external IPs
func TestReadAllWithLimit(t *testing.T)         // within limit, exceeding limit, empty body
func TestNewHTTPClient(t *testing.T)            // timeout and redirect policy
```

### 5. Add `internal/jobsvc/service_test.go`
**Impact: Medium | Effort: Medium**

```go
func TestNormalizeJobType(t *testing.T)         // table-driven normalization
func TestValidateJobType(t *testing.T)          // valid and invalid job types
func TestNormalizeProviderName(t *testing.T)    // case and whitespace handling
func TestExecuteJob(t *testing.T)              // sync and validate execution paths
```

### 6. Fix flaky `TestSpeedTestWithErrors`
**Impact: Low | Effort: Low**

The test at `internal/mirror/speedtest_test.go:107` expects at least one error
result but the condition is timing-dependent. Use a deterministic failure
injection instead of relying on network behavior.

### 7. Expand `internal/provider/ocp` tests (48.3% → 70%+)
**Impact: Medium | Effort: Medium**

This is the lowest-coverage package with actual logic. Focus on:
- RHCOS provider plan/sync logic
- Binary URL construction and validation
- Version filtering edge cases

### 8. Add table-driven patterns to existing tests
**Impact: Low | Effort: Low**

Convert repetitive test functions in `sqlite_test.go` and handler test files
into table-driven tests to improve readability and make it easier to add new
cases.

---

## Summary

| Category | Files | Lines of Untested Code |
|----------|-------|----------------------|
| Critical (must test) | 4 | ~2,070 |
| High priority | 4 | ~739 |
| Medium priority | 9 | ~1,439 |
| **Total untested** | **17** | **~4,248** |

The codebase has solid test foundations — the download client, sync engine, export
logic, and SQLite store are all well-tested. The main gaps are in the **HTTP
handler layer** (especially `handlers.go`), the **import engine**, and
**progress tracking**. Closing these gaps would bring the project to a much
more robust testing posture.
