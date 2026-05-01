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
hlctl containers list
hlctl containers list --device nas-1

# Container details
hlctl containers get nas-1.homeassistant

# Container lifecycle
hlctl containers start nas-1.homeassistant
hlctl containers stop nas-1.homeassistant
hlctl containers restart nas-1.homeassistant

# System health
hlctl system health
hlctl system info
hlctl system utilization

# Storage
hlctl storage volumes

# Backups
hlctl backups tasks

# Network
hlctl network devices
hlctl network clients

# JSON output
hlctl containers list -o json
```

## Domains

| Domain | Commands |
|--------|----------|
| `containers` | list, get, start, stop, restart |
| `system` | health, info, utilization, updates, update, check-updates |
| `storage` | volumes, volume |
| `backups` | tasks, task |
| `network` | devices, device, clients, client |
