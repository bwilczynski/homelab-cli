# hlctl

CLI for controlling homelab services via the [Homelab API](https://github.com/bwilczynski/homelab-api-spec).

## Prerequisites

- Go 1.22+
- GNU Make

## Setup

```sh
git clone --recurse-submodules https://github.com/bwilczynski/hlctl.git
cd hlctl
make generate
make build
```

## Configuration

```sh
# Set the API URL
hlctl config set-url https://homelab.local/api

# Verify
hlctl config show
```

Or use environment variables:

```sh
export HOMELAB_API_URL=https://homelab.local/api
export HOMELAB_TOKEN=your-token-here
```

## Usage

```sh
# List containers
hlctl docker containers list
hlctl docker containers list --device nas-1

# Container details
hlctl docker containers get nas-1.homeassistant

# Container lifecycle
hlctl docker containers start nas-1.homeassistant
hlctl docker containers stop nas-1.homeassistant
hlctl docker containers restart nas-1.homeassistant

# Docker networks
hlctl docker networks list
hlctl docker networks get <network-id>

# Docker images
hlctl docker images list
hlctl docker images get <image-id>

# System health
hlctl system health
hlctl system info
hlctl system utilization

# Storage
hlctl storage volumes list
hlctl storage backups list

# Network
hlctl network devices list
hlctl network clients list
hlctl network clients list --status offline

# JSON output
hlctl docker containers list -o json
```

## Domains

| Domain | Commands |
|--------|----------|
| `docker containers` | list, get, start, stop, restart |
| `docker networks` | list, get |
| `docker images` | list, get |
| `system` | health, info, utilization, updates, update, check-updates |
| `storage volumes` | list, get |
| `storage backups` | list, get |
| `network devices` | list, get |
| `network clients` | list, get |
