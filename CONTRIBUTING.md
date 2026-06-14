# Contributing to GitHub Notifier

## Development Setup

### Prerequisites
- Go 1.22+
- golangci-lint
- mockery

### Quick Start
```bash
git clone https://github.com/oak/github-notifier.git
cd github-notifier
go mod download
make install-tools
make test
make ci  # tests + lint + build
```

## Architecture

Hexagonal Architecture (Ports & Adapters) + DDD. See [ARCHITECTURE.md](ARCHITECTURE.md).

### Package Structure
- `domain/` — core business logic (no external dependencies)
- `application/` — use cases and ports (interfaces)
- `infrastructure/` — external adapters (GitHub API, notifications, persistence, UI)
- `cmd/` — entry points

## Making Changes

### 1. Branch
```bash
git checkout -b {feat,fix,...}/short-description
```

### 2. Code & Tests

For each layer: write failing test → implement → refactor.

**Coverage requirements:**
- Domain: >80%
- Application: >80%
- Infrastructure: >70%

**Test patterns:**
- Table-driven tests for multiple scenarios
- testify for assertions and mocking
- mockery for external dependencies

```bash
make test
make test-coverage
make test-race
```

### 3. Lint & Format
```bash
make lint
make fmt
```

### 4. Commit

Conventional commits:
```
feat: add support for draft PR filtering
fix: resolve race condition in tracking service
docs: update README with new configuration options
test: add tests for activity scheduler
refactor: simplify PR classification logic
```

### 5. Push & PR
```bash
git push origin feature/your-feature-name
```

**PR description:**
- **What** and **Why**
- **How** (complex changes only)
- **Testing**

**Checklist:**
- [ ] Tests added/updated
- [ ] Docs updated if needed
- [ ] `make ci` passes
- [ ] Commit messages follow conventions
- [ ] Branch up to date with master

## Example Test

```go
func TestPRFilter_FilterDrafts_ExcludesDrafts(t *testing.T) {
    filter := pullrequest.NewPRFilter(false)
    draftPR := testutil.NewTestPullRequest(1, testutil.WithDraft(true))
    normalPR := testutil.NewTestPullRequest(2, testutil.WithDraft(false))

    result := filter.FilterDrafts([]*pullrequest.PullRequest{draftPR, normalPR})

    require.Len(t, result, 1)
    assert.False(t, result[0].IsDraft())
}
```

## Common Tasks

For each step: write failing test → implement → refactor.

### Adding a New Feature
1. **Domain** — models and business logic
2. **Ports** — interfaces in `application/port/`
3. **Use Case** — orchestration in `application/usecase/`
4. **Adapters** — infrastructure in `infrastructure/`
5. **Wire Up** — `cmd/github-notifier/main.go`

### Adding a New Notification Channel
1. Create adapter in `infrastructure/notification/<channel>/`
2. Implement `NotificationPort` interface
3. Add to composition in `main.go`
4. Update docs

### Adding a New Domain Event
1. Define event type in `domain/pullrequest/events.go`
2. Create event constructor
3. Emit from aggregate
4. Create/update handler in `infrastructure/events/`
5. Subscribe in `main.go`

## Project Commands

```bash
make help              # show all commands
make test              # run tests
make test-coverage     # with coverage
make coverage-html     # HTML coverage report
make test-race         # with race detector
make lint              # run linter
make fmt               # format code
make build             # build
make clean             # clean artifacts
make install-tools     # install dev tools
make mocks             # generate mocks
make ci                # all CI checks
```