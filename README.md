# gomodtui

Browser-like TUI and CLI for [`pkg.go.dev`](https://pkg.go.dev) — search packages, view docs as markdown, and filter symbols without leaving the terminal. Built with [`charm.land/bubbletea/v2`](https://github.com/charmbracelet/bubbletea) and [`glamour`](https://github.com/charmbracelet/glamour).

![Go 1.27.1](https://img.shields.io/badge/go-1.27.1-00ADD8) ![Nix](https://img.shields.io/badge/nix-flake-5277C3)

## Features

* **Search** `pkg.go.dev` via official `GET /v1/search`
* **Package docs** via `GET /v1/package/{path}` rendered as markdown with `chroma` highlight (`dark` style)
* **Symbols** via `GET /v1/symbols/{path}` — filterable list (`s` in detail view, debounced)
* **Browser TUI** — independent full-screen panes for readability in tmux vertical splits, `hjkl` + mouse wheel, back stack (`esc`/`b`/`h`), no split cramping
* **CLI** — `search`, `package` (`get` alias), `tui`

## Prerequisites

* Nix with flakes enabled (`nix >= 2.18`, `experimental-features = nix-command flakes`)
* Or Go 1.27.1 for non-Nix builds

```shell
# enable flakes if not already
mkdir -p ~/.config/nix
echo "experimental-features = nix-command flakes" >> ~/.config/nix/nix.conf
```

## Install with Nix

### Run without installing

```shell
nix run github:FKouhai/gomodtui
# or specific ref
nix run github:FKouhai/gomodtui#gomodtui
# with args
nix run github:FKouhai/gomodtui -- search nats
nix run github:FKouhai/gomodtui -- tui
```

### Install to profile (imperative)

```shell
nix profile install github:FKouhai/gomodtui
# binary on PATH
gomodtui --help
gomodtui tui
gomodtui search tea
gomodtui package net/http --imports --doc md --json
# alias get works too
gomodtui get github.com/charmbracelet/bubbletea --json
```

Update / remove:

```shell
nix profile upgrade gomodtui
nix profile remove gomodtui
```

### Declarative (home-manager / nixos)

```nix
# flake.nix inputs
inputs.gomodtui.url = "github:FKouhai/gomodtui";

# then
home.packages = [ inputs.gomodtui.packages.${system}.default ];
# or nixos environment.systemPackages
```

### From source (cloned repo)

```shell
git clone https://github.com/FKouhai/gomodtui
cd gomodtui

# build
nix build
./result/bin/gomodtui --help
./result/bin/gomodtui tui

# with logs
nix build -L

# develop shell (Go 1.27.1 + gopls + gofumpt + staticcheck + govendor)
nix develop
# or with direnv
direnv allow
```

## Install without Nix

```shell
go install github.com/FKouhai/gomodtui@latest
# or from source
git clone https://github.com/FKouhai/gomodtui && cd gomodtui
go build -o /tmp/gomodtui
```

## Usage

```shell
# TUI browser (default when no subcommand)
gomodtui
gomodtui tui

# CLI
gomodtui search nats
gomodtui search "http client" --json

gomodtui package golang.org/x/time/rate --imports
gomodtui package net/http --doc md --json
gomodtui get charm.land/bubbletea/v2 --version v2.0.9
```

**TUI keys**

* `type` in top input → debounced search (300ms) → `↑↓`/`hjkl` navigate list → `enter` view docs (full-screen independent pane)
* Detail: `s` filter symbols, type to filter (120ms debounce, jumps to first match), `esc` clear filter, `hjkl`/`PgUp`/`wheel` scroll, `esc`/`b`/`h` back without exiting, `q`/`ctrl+c` quit, `/` focus search

## Development

```shell
nix develop
go vet ./...
go test ./... -race
go run . --help
go run . search tea
```

Pre-commit (`govendor` check) runs via `git-hooks.nix` in the dev shell.

## Acknowledgements

Bootstrapped with [go-overlay](https://github.com/purpleclay/go-overlay).
API docs at [`pkg.go.dev/api`](https://pkg.go.dev/api) and `pkg.go.dev/v1beta/openapi.yaml`.
