# Architecture

## Style

Hexagonal Architecture (Ports & Adapters) + DDD. Dependencies always point inward:

```
Infrastructure  →  Application  →  Domain
```

- **Domain** — pure business logic, zero external dependencies
- **Application** — use cases that orchestrate domain operations; defines ports (interfaces)
- **Infrastructure** — concrete adapters for GitHub, notifications, UI, persistence

---

## Package Tree

```
github-notifier/
├── cmd/github-notifier/
│   ├── main.go                        # Composition root — wires all dependencies, starts systray
│   ├── platform_darwin.go             # Darwin: UNUserNotificationCenter → terminal-notifier → beeep fallback chain
│   └── platform_default.go            # All other platforms: default adapter selection
│
├── domain/pullrequest/                # Core domain — no external dependencies
│   ├── aggregate.go                   # PullRequest aggregate root
│   ├── value_objects.go               # PRIdentifier, RepositoryInfo, Author, PRStatus
│   ├── pipeline_status.go             # PipelineStatus (Unknown, Running, Success, Failed)
│   ├── review.go                      # Review, ReviewSummary, ReviewState
│   ├── events.go                      # Domain events + event name constants
│   ├── draft_filter.go                # FilterFn type + NewDraftFilter
│   ├── classifiers.go                 # ClassifyPRs — own vs. requested-review classification
│   ├── activity_scheduler.go          # ActivityCheckScheduler — two-tier recent/stale logic
│   ├── activity.go                    # Activity value object (comments, reviews, commits)
│   ├── ignore.go                      # IgnoreConfig, IgnoreRuleSet, IgnoreScope, IgnoreActorRule
│   ├── ignore_filter.go               # ActivityIgnoreFilter — evaluates IgnoreConfig rules
│   ├── repositories.go                # PullRequestRepository + PRTrackingRepository ports
│   └── errors.go                      # Domain error types
│
├── application/
│   ├── orchestrator.go                # PullRequestOrchestrator — coordinates the 5 use cases
│   ├── port/
│   │   ├── event_handler.go           # EventHandler interface
│   │   ├── event_publisher.go         # EventPublisher interface
│   │   ├── notification.go            # NotificationPort interface + DTOs
│   │   └── ui.go                      # UIPort interface
│   └── usecase/
│       ├── initialize_first_check.go  # First run: mark all existing PRs seen silently
│       ├── check_new_pull_requests.go # Fetch PRs, detect new ones, publish events
│       ├── detect_closed_pull_requests.go # Detect merged/closed, publish events
│       ├── track_pull_request_activity.go # Two-tier activity checking, publish events
│       └── update_pull_request_display.go # Refresh systray menu
│
├── infrastructure/
│   ├── config/
│   │   ├── config.go                  # Loads config from file + env vars
│   │   ├── editor.go                  # Opens config/ignore files in default editor
│   │   ├── watcher.go                 # Watches config file for a valid token (fsnotify)
│   │   ├── ignore.go                  # ignore.yaml default template generation
│   │   ├── ignore_types.go            # Private yaml DTOs + toDomain() mapper
│   │   ├── ignore_loader.go           # Loads and parses ignore.yaml → *pullrequest.IgnoreConfig
│   │   └── ignore_watcher.go          # Polls ignore.yaml for changes
│   ├── events/
│   │   ├── bus.go                     # InMemoryEventBus — implements EventPublisher
│   │   ├── notification_aggregator.go # Batches events before flushing to NotificationPort
│   │   ├── notification_handler.go    # NotificationEventHandler
│   │   └── tracking_handler.go        # TrackingEventHandler
│   ├── github/
│   │   ├── adapter.go                 # Implements PullRequestRepository (GitHub GraphQL)
│   │   ├── client.go                  # HTTP/GraphQL client wrapper
│   │   ├── mapper.go                  # GitHub DTOs → domain entities
│   │   └── dto.go                     # GitHub API response types
│   ├── notification/
│   │   ├── composite_adapter.go       # Fans out to multiple adapters
│   │   ├── desktop/adapter.go         # beeep cross-platform desktop notifications
│   │   ├── linux/adapter.go           # esiqveland/notify D-Bus notifications
│   │   ├── macos/adapter.go           # terminal-notifier with click actions
│   │   ├── macos/un/adapter.go        # Native UNUserNotificationCenter via CGo
│   │   └── slack/adapter.go           # nikoksr/notify Slack DM notifications
│   ├── persistence/
│   │   ├── models.go                  # PRStateSnapshot + ReviewSnapshot
│   │   ├── memory/
│   │   │   └── pr_tracking_repository.go
│   │   └── json/
│   │       └── state_repository.go    # JSON-file-backed PRTrackingRepository
│   └── ui/
│       ├── menu_adapter.go            # Implements UIPort — systray menu
│       ├── formatter.go               # Presentation helpers
│       └── theme_provider.go          # Detects dark/light system theme
│
├── assets/
│   └── embed.go                       # go:embed declarations for bundled icons
│
└── internal/
    ├── logger/logger.go               # zerolog initialisation
    ├── mocks/                         # mockery-generated mocks for all ports
    └── testutil/                      # Shared test fixtures and helpers
```

---

## Domain Model

### Aggregate Root

`PullRequest` (`domain/pullrequest/aggregate.go`) is the single aggregate root. It holds PR metadata, status, review state, pipeline status, activity history, and the draft flag. All mutations go through aggregate methods.

### Value Objects

| Type | File | Description |
|------|------|-------------|
| `PRIdentifier` | `value_objects.go` | URL + number — unique PR identity |
| `RepositoryInfo` | `value_objects.go` | GitHub repository (owner + name) |
| `Author` | `value_objects.go` | GitHub user login |
| `PRStatus` | `value_objects.go` | Open / Merged / Closed |
| `Activity` | `activity.go` | A comment, review, or commit on a PR |
| `ReviewState` | `review.go` | Approved / ChangesRequested / Commented / Dismissed |
| `Review` | `review.go` | A single reviewer's state + timestamp |
| `PipelineStatus` | `pipeline_status.go` | Unknown / Running / Success / Failed |
| `IgnoreConfig` | `ignore.go` | Pure domain type for ignore rules (no yaml tags) |

### Domain Services

| Service | File | Responsibility |
|---------|------|----------------|
| `NewDraftFilter` / `FilterFn` | `draft_filter.go` | Returns a function that strips draft PRs (or a no-op) |
| `ClassifyPRs` | `classifiers.go` | Splits PRs into "truly new" vs "has activity" |
| `ActivityCheckScheduler` | `activity_scheduler.go` | Decides which PRs need activity checking this cycle |
| `ActivityIgnoreFilter` | `ignore_filter.go` | Returns true if an event should be suppressed per IgnoreConfig |

### Domain Events

All events implement `Event` (`OccurredAt() time.Time`).

| Constant | Struct | Raised when |
|----------|--------|-------------|
| `EventNewPullRequestDetected` | `NewPullRequestDetected` | PR seen for the first time |
| `EventActivityDetected` | `ActivityDetected` | New comment, review, or commit |
| `EventReviewStateChanged` | `ReviewStateChanged` | Reviewer state changes |
| `EventMerged` | `Merged` | PR is merged |
| `EventClosed` | `Closed` | PR is closed without merging |
| `EventPipelineStatusChanged` | `PipelineStatusChanged` | CI/CD status changes |
| `EventStatusChanged` | `StatusChanged` | PR open/merged/closed status changes |

---

## Application Layer

### Orchestrator

`application/orchestrator.go` runs on every tick:

```
ExecuteRegularCheck()
  │
  ├─ 1. CheckNewPullRequestsUseCase       — fetch PRs, detect new → publish NewPullRequestDetected
  ├─ 2. DetectClosedPullRequestsUseCase   — diff snapshot → publish Merged / Closed
  ├─ 3. TrackPullRequestActivityUseCase   — two-tier activity check → publish ActivityDetected etc.
  └─ 4. UpdatePullRequestDisplayUseCase   — refresh systray via UIPort
```

On startup, `ExecuteInitialCheck()` runs `InitializeFirstCheckUseCase` (marks all existing PRs seen silently), then falls through to `ExecuteRegularCheck` if state already exists.

### Use Cases

| Use Case | File | Responsibility |
|----------|------|----------------|
| `InitializeFirstCheckUseCase` | `initialize_first_check.go` | First run: mark all current PRs seen without notifying |
| `CheckNewPullRequestsUseCase` | `check_new_pull_requests.go` | Fetch review-requested + user PRs; detect new; publish events |
| `DetectClosedPullRequestsUseCase` | `detect_closed_pull_requests.go` | Diff current vs. tracked snapshot; detect merged/closed |
| `TrackPullRequestActivityUseCase` | `track_pull_request_activity.go` | Enrich PRs with activities (two-tier); publish events |
| `UpdatePullRequestDisplayUseCase` | `update_pull_request_display.go` | Call `UIPort.UpdateDisplay` with latest PR state |

---

## Ports

### Domain Ports (`domain/pullrequest/repositories.go`)

```go
type PullRequestRepository interface {
    FetchRequestedReviews() ([]*PullRequest, error)
    FetchUserCreated() ([]*PullRequest, error)
    EnrichWithActivities(prs []*PullRequest, since time.Time) ([]Event, error)
    FetchPRStatus(owner, repo string, number int) (PRStatus, error)
    AuthenticatedUser() string
}

type PRTrackingRepository interface {
    Fetch(prIdentifier PRIdentifier) (*PullRequest, error)
    LoadAll() ([]*PullRequest, error)
    Update(pullRequest *PullRequest) error
    Save(pullRequests []*PullRequest) error
    IsEmpty() bool
    Clear() error
}
```

### Application Ports (`application/port/`)

```go
type EventPublisher interface {
    Publish(event pullrequest.Event) error
}

type EventHandler interface {
    Handle(ctx context.Context, event pullrequest.Event) error
}

type NotificationPort interface {
    NotifyPullRequests(notifications []*PRNotificationData) error
    NotifyMessage(title, message string) error
    SupportsClickActions() bool
}

type UIPort interface {
    UpdateDisplay(requestedReviewPRs, userCreatedPRs []*pullrequest.PullRequest, trackingRepo pullrequest.PRTrackingRepository)
}
```

---

## Event Flow

Events go through `InMemoryEventBus` (`infrastructure/events/bus.go`).

| Event | Handlers |
|-------|----------|
| `NewPullRequestDetected` | `NotificationEventHandler`, `TrackingEventHandler` |
| `ActivityDetected` | `NotificationEventHandler`, `TrackingEventHandler` |
| `ReviewStateChanged` | `NotificationEventHandler`, `TrackingEventHandler` |
| `Merged` | `NotificationEventHandler`, `TrackingEventHandler` |
| `Closed` | `NotificationEventHandler`, `TrackingEventHandler` |
| `PipelineStatusChanged` | `NotificationEventHandler`, `TrackingEventHandler` |

`NotificationEventHandler` feeds events into `NotificationAggregator`, which batches them for 2 seconds then flushes a single grouped call to `NotificationPort` — avoids storms when many PRs change at once.

`TrackingEventHandler` updates `PRTrackingRepository` — e.g. marking a PR seen so the `*` disappears from the menu.

---

## Data Flow

```
cmd/main.go  (every tick)
     │
     ▼
PullRequestOrchestrator
     │
     ├─► CheckNewPullRequestsUseCase
     │       │ FetchRequestedReviews()  ┐
     │       │ FetchUserCreated()       ├─ PullRequestRepository (GitHub adapter)
     │       └─ Publish(NewPRDetected) ─┴──► InMemoryEventBus
     │                                            │
     ├─► DetectClosedPullRequestsUseCase          ├──► NotificationEventHandler
     │       │ FetchPRStatus()                    │       └─► NotificationAggregator
     │       └─ Publish(Merged / Closed)          │               └─► NotificationPort
     │                                            │
     ├─► TrackPullRequestActivityUseCase          └──► TrackingEventHandler
     │       │ ActivityCheckScheduler                      └─► PRTrackingRepository
     │       └─ Publish(ActivityDetected / ...)
     │
     └─► UpdatePullRequestDisplayUseCase
             └─► UIPort.UpdateDisplay()
                     └─► MenuAdapter (systray)
```

---

## How to Extend

### Add a Notification Channel

For each step: write failing test → implement → refactor.

1. Create `infrastructure/notification/<channel>/adapter.go`
2. Implement `port.NotificationPort`
3. In `main.go`, wrap with `notification.NewCompositeAdapter(existing, newAdapter)`

The Slack adapter is a reference implementation.

### Add a UI Implementation

1. Create `infrastructure/ui/<name>/adapter.go`
2. Implement `port.UIPort`
3. Inject in `main.go` in place of (or alongside) `MenuAdapter`

Possible: web UI, terminal TUI, CLI output.

### Add a PR Source

1. Create adapter in `infrastructure/`
2. Implement `pullrequest.PullRequestRepository`
3. Inject in `main.go`, or wrap multiple sources with a composite adapter

### Add a Persistence Backend

A JSON-file implementation exists at `infrastructure/persistence/json/state_repository.go`. To add another:

1. Create `infrastructure/persistence/<engine>/pr_tracking_repository.go`
2. Implement `pullrequest.PRTrackingRepository`
3. Replace the existing adapter in `main.go`

### Add a Domain Event

1. Define the event struct in `domain/pullrequest/events.go`
2. Add an `EventName` constant
3. Emit from the aggregate or a use case
4. Add a handler case in `NotificationEventHandler` and/or `TrackingEventHandler`
5. Subscribe in `main.go`

---

## Testing Strategy

| Layer | Approach |
|-------|----------|
| `domain/` | Pure unit tests — no mocks, no external dependencies |
| `application/usecase/` | Unit tests with mockery-generated mocks for all ports |
| Infrastructure adapters | Integration tests where practical (mapper, event bus) |
| End-to-end | `test/e2e/github_flow_test.go` — full flow with stubbed GitHub adapter |

```bash
make test           # all tests
make test-coverage  # with coverage report
make test-race      # with race detector
make mocks          # regenerate after changing any port interface
```

---

## Platform Notes

| Platform | Notification adapter | Theme detection | Systray |
|----------|---------------------|-----------------|---------|
| **Linux** | `linux/` (libnotify) | D-Bus portal | GTK3 + libappindicator3 |
| **macOS** | `macos/` (terminal-notifier) + `macos/un/` (UNUserNotificationCenter via CGo, preferred in .app bundle) | `defaults read` | Native |
| **Windows** | `desktop/` (beeep) | PowerShell registry | Native |

Adapter selection happens at runtime in `main.go` — no compile-time flags needed.