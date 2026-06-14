# GitHub Notifier

[![CI](https://github.com/oak/github-notifier/actions/workflows/ci.yml/badge.svg)](https://github.com/oak/github-notifier/actions)
[![codecov](https://codecov.io/gh/oak/github-notifier/branch/master/graph/badge.svg)](https://codecov.io/gh/oak/github-notifier)
[![Go Report Card](https://goreportcard.com/badge/github.com/oak/github-notifier)](https://goreportcard.com/report/github.com/oak/github-notifier)

System tray app that monitors GitHub pull requests and sends desktop notifications. Built to explore Go's concurrency model, cross-platform GUI patterns, and efficient API design.

## Goals

- Practice idiomatic Go: channels, goroutines, interface-driven design
- Integrate with OS-native APIs (systray, desktop notifications) across Linux/macOS/Windows
- Minimize GitHub API usage via batched GraphQL and tiered polling
- Ship something actually useful day-to-day

## Features

- **System tray** — runs silently in the background
- **Desktop notifications** — native alerts for new PRs and activity
- **Participation tracking** — tracks PRs you've reviewed, not just open review requests
- **Activity tracking** *(optional)* — comments, reviews, and commits on your PRs
- **Slack integration** *(optional)* — DM notifications via Slack bot
- **Theme-aware icons** — auto-switches dark/light based on system theme
- **Optimised API usage** — batched GraphQL cuts total API calls
- **Multi-platform** — Linux, macOS, Windows

## Prerequisites

- **Go 1.22+** — [golang.org/dl](https://golang.org/dl/)
- **GitHub Personal Access Token** — [create one](https://github.com/settings/tokens) with `repo:read` and `read:user`
- **GTK3** *(Linux only)*:

```bash
# Ubuntu/Debian
sudo apt-get install libgtk-3-dev libappindicator3-dev

# Fedora/RHEL
sudo dnf install gtk3-devel libappindicator-gtk3-devel

# Arch
sudo pacman -S gtk3 libappindicator-gtk3
```

## Quick Start

```bash
git clone https://github.com/oak/github-notifier.git
cd github-notifier
go build -o github-notifier ./cmd/github-notifier
export GITHUB_TOKEN="your_token_here"
./github-notifier
```

First run creates `~/.github-notifier.conf` and opens it in your editor.

## Configuration

Loaded from `~/.github-notifier.conf`; environment variables override.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GITHUB_TOKEN` | ✓ | — | GitHub personal access token |
| `SLACK_OAUTH_TOKEN` | | — | Slack bot token for DM notifications |
| `CHECK_INTERVAL_MINUTES` | | `1` | Minutes between checks (1–60) |
| `ENABLE_ACTIVITY_TRACKING` | | `false` | Track comments, reviews, commits |
| `RECENT_PR_THRESHOLD_HOURS` | | `72` | PRs younger than this check every minute |
| `STALE_PR_CHECK_INTERVAL` | | `15` | Check stale PRs every N minutes |
| `INCLUDE_DRAFT_PRS` | | `true` | Include drafts you've participated in |
| `MACOS_NOTIFICATION_SENDER` | | — | macOS notification bundle ID |

## How It Works

### Participation Tracking

Two queries run each cycle:
- `is:open is:pr review-requested:@me`
- `is:open is:pr reviewed-by:@me`

The second query matters: GitHub removes you from "requested reviewers" once you approve, so without it you'd silently lose visibility on PRs you already reviewed but are still open.

### Activity Tracking

When enabled, uses a two-tier polling strategy to reduce API load:

| Tier | Condition | Frequency |
|------|-----------|-----------|
| Recent | PR < 72h old | Every minute |
| Stale | PR ≥ 72h old | Every 15 minutes |

PRs with unseen activity show `*` in the tray menu until clicked.

## Docs

- [ARCHITECTURE.md](ARCHITECTURE.md) — structure, design decisions, extension guide
- [CONTRIBUTING.md](CONTRIBUTING.md) — dev workflow and conventions

## License

MIT