# hlctl

One CLI for the whole homelab: Docker hosts, storage, system health, UniFi gear. Built on the [Homelab API](https://github.com/bwilczynski/homelab-api-spec).

## The party trick

Ever wished you could `tree` your network? You can.

```sh
$ hlctl network topology --include-wireless
UCG-Fiber (gateway)
└── Switch-Core (switch) [port 1, 10 GbE]
    ├── NAS (client, wired, online) [port 1, 10 GbE]
    ├── Workstation (client, wired, online) [port 2, 2.5 GbE]
    ├── AP-Office (accessPoint) [port 3, 2.5 GbE] [3 clients]
    │   ├── MacBook-Pro (client, wireless, online) [HomeWiFi, -48 dBm]
    │   ├── Pixel-9 (client, wireless, online) [HomeWiFi, -55 dBm]
    │   └── Kindle (client, wireless, online) [IoT, -71 dBm]
    ├── AP-Living-Room (accessPoint) [port 4, 2.5 GbE] [4 clients]
    │   ├── iPhone (client, wireless, online) [HomeWiFi, -52 dBm]
    │   ├── iPad-Kitchen (client, wireless, online) [HomeWiFi, -65 dBm]
    │   ├── AppleTV (client, wireless, online) [HomeWiFi, -41 dBm]
    │   └── Roborock (client, wireless, online) [IoT, -68 dBm]
    └── Switch-Flex (switch) [port 6, 2.5 GbE]
        ├── HomeAssistant-Yellow (client, wired, online) [port 2, 1 GbE]
        ├── Printer (client, wired, online) [port 3, 100 Mbps]
        └── Camera-Garage (client, wired, online) [port 4, 100 Mbps]
```

What you're looking at: a UCG Fiber gateway, a UniFi Pro XG 8 PoE 10G core switch, two U7 Pro XG access points, and a Flex 2.5G PoE, all tucked into a single 10-inch DeskPi mini rack. Port numbers and link speeds are live, pulled from the UniFi controller. Skip `--include-wireless` if you only want the wired infrastructure, or add `-o json` to pipe it into something else.

## Zooming in on a switch

Same data, one device at a time. Live port stats, PoE draw, per-port traffic, who's on the other end of each cable.

```sh
$ hlctl network devices get unifi.usw-pro-xg-8-poe
FIELD       VALUE
ID          unifi.usw-pro-xg-8-poe
NAME        Switch-Core
MAC         74:83:c2:11:22:33
IP          192.168.1.2
TYPE        switch
STATUS      connected
MODEL       USWProXG8POE
FIRMWARE    7.4.1.16850
UPTIME      42d 6h 18m
TRAFFIC RX  18.4 MB/s (8.4 TB total)
TRAFFIC TX  6.1 MB/s (5.2 TB total)
UPLINK      UCG-Fiber

--- PORTS ---
PORT  STATE  SPEED    POE   POE WATTS  RX         TX         CONNECTED TO
1     up     10 GbE   off   -          12.8 MB/s  3.4 MB/s   NAS
2     up     2.5 GbE  off   -          0.4 MB/s   1.2 MB/s   Workstation
3     up     2.5 GbE  auto  8.4 W      0.6 MB/s   2.1 MB/s   AP-Office
4     up     2.5 GbE  auto  9.2 W      1.1 MB/s   4.8 MB/s   AP-Living-Room
5     down   -        off   -          0 B/s      0 B/s      -
6     up     2.5 GbE  auto  6.1 W      0.3 MB/s   0.9 MB/s   Switch-Flex
7     down   -        off   -          0 B/s      0 B/s      -
8     down   -        off   -          0 B/s      0 B/s      -
```

Useful when an AP is suddenly drawing 14W instead of the usual 8, or you want to know why that camera in the garage negotiated 100 Mbps on a cable that should do gigabit.

## Patch Tuesday, every day

Every container on every host, its installed version, the latest upstream. When a CVE drops, you don't have to SSH into three boxes to find out if you're exposed.

```sh
$ hlctl system updates list --status updateAvailable
ID                   NAME            DEVICE  TYPE       STATUS           CURRENT   LATEST
nas-1.vaultwarden    Vaultwarden     nas-1   container  updateAvailable  1.32.7    1.34.0
nas-1.homeassistant  Home Assistant  nas-1   container  updateAvailable  2026.4.2  2026.5.1
nas-1.paperless-ngx  Paperless-ngx   nas-1   container  updateAvailable  2.13.5    2.14.0
nas-2.gitea          Gitea           nas-2   container  updateAvailable  1.22.6    1.23.0
```

Drop the filter to see the up-to-date stuff too, or drill in when a version bump catches your eye:

```sh
$ hlctl system updates get nas-1.vaultwarden
FIELD         VALUE
ID            nas-1.vaultwarden
NAME          Vaultwarden
DEVICE        nas-1
TYPE          container
STATUS        updateAvailable
CURRENT       1.32.7
LATEST        1.34.0
CHECKED AT    2026-05-18T08:30:00+02:00
PUBLISHED AT  2026-04-25T16:00:00+02:00
IMAGE         vaultwarden/server:1.32.7
SOURCE        https://github.com/dani-garcia/vaultwarden
RELEASE URL   https://github.com/dani-garcia/vaultwarden/releases/tag/1.34.0
```

One click on the release URL tells you whether it's a security patch worth a maintenance window tonight or just feature noise that can wait for the weekend.

## Domains

| Domain | What's in it |
|--------|----------------|
| `docker` | Containers (lifecycle + inspect), networks, images, across every host |
| `system` | Health, info, utilization, OS updates |
| `storage` | Volumes and backups |
| `network` | UniFi devices, clients, full topology |
| `config` / `login` | Local CLI configuration and OAuth2 auth |

Every command takes `--output json` and `--help`. Start from `hlctl --help`.

## Setup

Requires Go 1.22+ and GNU Make.

```sh
git clone --recurse-submodules https://github.com/bwilczynski/hlctl.git
cd hlctl
make generate
make build
```

## Configuration

```sh
hlctl config set-url https://homelab.local/api
hlctl login
hlctl config show
```

Or set `HOMELAB_API_URL` and `HOMELAB_TOKEN` per shell. Config lives at `~/.config/homelab/`.
