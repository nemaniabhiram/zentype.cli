# ZenType CLI (`zt`)

A minimal terminal-based typing speed test. Measure words-per-minute (WPM) and accuracy directly from your shell.

## Installation

Requires Go 1.21+. [Install Go](https://go.dev/dl/).

```bash
# Install latest released commit
go install github.com/nemaniabhiram/zentype.cli/zt@latest

# If the proxy has not refreshed yet, install from the main branch
# go install github.com/nemaniabhiram/zentype.cli/zt@main
```

The binary `zt` is placed in `$GOBIN` (default `%USERPROFILE%\go\bin` on Windows or `$HOME/go/bin` on Unix-like systems). Ensure this directory is on your `PATH`.

## Quick Start

```bash
# Start a 60-second test (default)
zt

# Custom duration
zt --time 30

# Other commands
zt --leaderboard    # view global leaderboard
zt auth             # authenticate with GitHub
zt version          # print version
```

## Commands

| Command | Description |
|---------|-------------|
| `zt` | Start a 60-second typing test |
| `zt --time <seconds>` | Custom duration test (10-300 s) |
| `zt --leaderboard` | Show global leaderboard / your rank |
| `zt auth [--logout|--status]` | Authenticate with GitHub, logout, or show status |
| `zt version` | Print the current version |

## Keybindings (during test)

| Key | Action |
|-----|--------|
| `Esc` / `Ctrl+C` | Quit application |
| `Enter` | Restart test |

## Contributing

1. Fork the repository and clone your fork.
2. Ensure `go vet ./...` passes before submitting a pull request.
3. Use conventional commit messages (`feat: …`, `fix: …`, etc.).

## Development

```bash
# Run without installing
go run ./zt --time 45

# Tidy dependencies
go mod tidy
```