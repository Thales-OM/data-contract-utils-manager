<div align="center">

# data-contract-utils-manager

A tiny, self-contained version manager for teams that ship their internal
CLI tooling as GitLab releases — download once, cache locally, switch
versions instantly.

[![CI](https://github.com/Thales-OM/data-contract-utils-manager/actions/workflows/ci.yml/badge.svg)](https://github.com/Thales-OM/data-contract-utils-manager/actions/workflows/ci.yml)
[![Release](https://github.com/Thales-OM/data-contract-utils-manager/actions/workflows/release.yml/badge.svg)](https://github.com/Thales-OM/data-contract-utils-manager/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Thales-OM/data-contract-utils-manager)](https://goreportcard.com/report/github.com/Thales-OM/data-contract-utils-manager)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

</div>

## Why

When internal lint/build tools are distributed through a private GitLab
instance, keeping the right version on every machine gets tedious: release
assets sit behind authentication, package URLs need rewriting, and nobody
remembers which build is currently installed.

`data-contract-utils-manager` solves this with one static binary that:

- lists published releases of any GitLab project,
- downloads the executable asset for the requested version,
- caches it next to its own binary so re-switching is offline and instant,
- exposes the active version from a stable `current/` path you can put on
  your `PATH`.

## Features

- **Cache-first switching** — repeated `switch` calls hit the local cache,
  no network round-trip.
- **Interactive asset picker** — multi-platform releases prompt for the
  right binary; `--name` skips the prompt in scripts/CI.
- **Token handling done right** — personal access tokens are stored in a
  local `.env`, never logged, and sent only as `PRIVATE-TOKEN`.
- **Configurable endpoints** — point it at any GitLab instance (on-prem or
  gitlab.com) via environment variables; nothing is hardcoded.
- **Self-contained state** — everything lives beside the executable:
  copy one file to a machine and it works.

## Installation

### Download a release (recommended)

Grab the archive for your platform from the
[Releases page](https://github.com/Thales-OM/data-contract-utils-manager/releases),
unpack it and place the binary anywhere convenient:

```console
$ tar -xzf data-contract-utils-manager_linux_amd64.tar.gz
$ sudo mv data-contract-utils-manager /usr/local/bin/
```

> Tip: the name is long — feel free to rename the binary to `dcum`
> (`mv data-contract-utils-manager /usr/local/bin/dcum`); all state paths
> follow the executable, so it stays self-contained.

### From source

```console
$ go install github.com/Thales-OM/data-contract-utils-manager@latest
```

Requires Go 1.24+.

## Quick start

```console
# One-time setup: where are the releases?
$ dcum set-token <personal-access-token>        # stored in .env next to the binary
$ export GITLAB_BASE_URL=https://gitlab.example.com
$ export GITLAB_PROJECT_ID=20968                # or put both into .env

# See what is available and what is installed
$ dcum list
Releases of project 20968:
* v1.4.0      Data Contract Utils v1.4.0               2026-03-02
+ v1.3.9      Data Contract Utils v1.3.9               2026-02-11
  v1.2.8      Data Contract Utils v1.2.8               2025-12-15

(* active, + cached)

Cached assets (.../cache):
  dcu-1.3.9-windows-amd64.exe

Active version: v1.4.0

# Switch versions
$ dcum switch v1.3.9
Version v1.3.9 found in cache.
Switched to version v1.3.9. Executable placed at .../current/dcu.exe
```

Add `<install-dir>/current` to your `PATH` and the active tool is always
available under its plain name (`dcu`), regardless of which version is
behind it.

## Commands

| Command | Description |
| ------- | ----------- |
| `switch <version>` | Download (or reuse) a release and activate it |
| `list` | Show remote releases plus local cache state |
| `set-token <token>` | Persist a GitLab token into the local `.env` |
| `version` | Print build information |

Useful flags on `switch`:

| Flag | Effect |
| ---- | ------ |
| `--name <asset>` | Pick a specific release asset instead of prompting |
| `--force` | Re-download even if the version is already cached |
| `--token <token>` | Override the stored token for a single call |

## Configuration

Settings resolve in order **flag → environment variable → `.env` file**.

| Variable | Required | Description |
| -------- | -------- | ----------- |
| `GITLAB_BASE_URL` | yes | Root URL of the GitLab instance, e.g. `https://gitlab.example.com` |
| `GITLAB_PROJECT_ID` | yes | Numeric project ID or URL-encoded `group/project` path |
| `GITLAB_TOKEN` | no* | Personal access token with `read_api` scope |

\* Optional only for fully public projects — private instances need it.

Example `.env` (kept next to the executable, gitignored by default):

```dotenv
GITLAB_BASE_URL=https://gitlab.example.com
GITLAB_PROJECT_ID=20968
GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
```

## How it works

All state lives beside the executable, making the tool fully portable:

```
<install-dir>/
├── data-contract-utils-manager(.exe)   # the manager itself
├── .env                                # persisted settings (optional)
├── cache/                              # downloaded assets, kept forever
│   ├── meta.yaml                       # tag -> asset map + active version
│   ├── dcu-1.3.9-windows-amd64.exe
│   └── dcu-1.4.0-windows-amd64.exe
└── current/                            # active version, simplified name
    └── dcu.exe
```

`switch` records the mapping in `meta.yaml`, copies the chosen asset to
`current/<tool>.exe` (version suffixes stripped), and Unix builds get the
executable bit set automatically.

Release assets published through GitLab's *generic package registry*
(`package_files/...` download links) are transparently rewritten to the
API endpoint that accepts token authentication.

## Development

```console
$ make test     # race-enabled unit tests
$ make lint     # golangci-lint
$ make build    # binary in bin/, version stamped from git describe
```

The project layout follows standard Go conventions:

```
cmd/              cobra command definitions and CLI flow
internal/config   settings resolution (flags > env > .env)
internal/gitlab   minimal releases API client + URL helpers
internal/store    cache index, asset storage, activation logic
```

CI runs tests on Linux, Windows and macOS; pushing a `v*` tag publishes
cross-built binaries via GoReleaser.

## License

[MIT](LICENSE)
