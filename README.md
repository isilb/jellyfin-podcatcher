# Jellyfin Podcatcher Sync Daemon

A bare-metal, high-performance Go daemon designed to run continuously as a native system process (or inside an unprivileged Proxmox LXC container). It automatically polls public, tokenized, or premium podcast RSS feeds every hour, streams missing media assets, writes local Kodi-compliant metadata files (`.nfo`), and fires a library re-index webhook straight to your Jellyfin server.

It includes an integrated **Jellyfin Favorites Deletion Shield** that cross-references user data to protect starred or favorited episodes from automatic chronological cleanup cycles.

## Features

- **Host-Native Efficiency:** Eliminates heavy Docker layers or runtime container virtualization—compiles down to a single binary managed by a lightweight systemd loop.
- **Dynamic Hot-Reloading:** Automatically reloads configurations from `config.json` at the start of each hourly execution sweep. Feeds can be added or removed dynamically without restarting the underlying service.
- **Jellyfin Star Shield:** Queries the Jellyfin server API before carrying out any retention purges. Episodes marked with a "Favorite" star/heart icon by *any* user profile on *any* client app are preserved.
- **Automatic Deduplication:** Uses file-system checking (`os.Stat`) to verify asset filenames on disk before initializing data streams, preserving local network bandwidth.
- **Kodi-Compliant Metadata Generation:** Automatically compiles companion XML `.nfo` files for ingested items, allowing Jellyfin to instantly display detailed descriptions without querying external web scrapers.

---

## Architecture Flow

```text
  [Systemd Service Unit] -> Environment Secrets Injected at Runtime
         │
         ▼
  [main.go Daemon Loop] (Executes every 60 minutes)
         │
         ├─► Hot-Reloads [config.json] for Active RSS Feed Matrix
         │
         ├─► Interrogates Jellyfin Server -> Caches "IsFavorite" File Paths
         │
         ├─► Runs Pruning Engine -> Evicts Items Exceeding Max Age Limit
         │
         ├─► Audits Disk (os.Stat) -> Skips Existing Tracks (Zero Duplicates)
         │
         ├─► Streams Missing MP3 Payloads -> Generates Metadata Sidecar NFOs
         │
         └─► Outbound Webhook -> Forces Automated Jellyfin Library Refresh
```
## Deployment & Setup

### 1. Position File Structures
Clone or place your working source files into the target runtime directory:

```bash
mkdir -p /opt/jellyfin-podcatcher
cd /opt/jellyfin-podcatcher
```


