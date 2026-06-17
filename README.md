# Media Server Podcast Sync Daemon

<img width="500" height="500" alt="image" src="https://github.com/user-attachments/assets/ab54bf2c-8ada-4cfe-906a-00f8a91af763" />

A bare-metal or containerized, high-performance Go daemon designed to run continuously as a native system process. It automatically polls public, tokenized, or premium podcast RSS feeds every hour, streams missing media assets, writes local Kodi-compliant metadata files (`.nfo`), and fires a library re-index webhook straight to your home media server.  Tested most extensively with Jellyfin, but works for any media server like Emby or Plex.
## Features

- **Host-Native Efficiency:** Eliminates heavy Docker layers or runtime container virtualization—compiles down to a single binary managed by a lightweight systemd loop.
- **Dynamic Hot-Reloading:** Automatically reloads configurations from `config.json` at the start of each hourly execution sweep. Feeds can be added or removed dynamically without restarting the underlying service.
- **Favourites Shield:** Queries the Jellyfin/Emby/Plex server API before carrying out any retention purges. Episodes marked with a "Favorite" star/heart icon by *any* user profile on *any* client app are preserved.
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
         ├─► Interrogates Media Server -> Caches "IsFavorite" File Paths
         │
         ├─► Runs Pruning Engine -> Evicts Items Exceeding Max Age Limit
         │
         ├─► Audits Disk (os.Stat) -> Skips Existing Tracks (Zero Duplicates)
         │
         ├─► Streams Missing MP3 Payloads -> Generates Metadata Sidecar NFOs
         │
         └─► Outbound Webhook -> Forces Automated Jellyfin/Emby/Plex Library Refresh
```
## Deployment & Setup

### Prerequisite: Install Go Toolchain
The daemon must be compiled from source. If Go is not installed on your host or LXC container, run the following commands to install it natively:

```bash
# Download the Go tarball
wget [https://go.dev/dl/go1.22.4.linux-amd64.tar.gz](https://go.dev/dl/go1.22.4.linux-amd64.tar.gz)

# Extract it to /usr/local
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.4.linux-amd64.tar.gz

# Add Go to your system PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### 1. Position File Structures
Clone or place your working source files into the target runtime directory:

```bash
mkdir -p /opt/jellyfin-podcatcher && cd /opt/jellyfin-podcatcher
git clone https://github.com/isilb/jellyfin-podcatcher.git
```

### 2. Configure Your Feed Array
Create a clear, non-sensitive config.json tracking array. This profile handles feed structures and contains no keys or hardcoded paths, making it completely safe to track publicly:

```bash
{
  "feeds": [
    {
      "name": "",
      "url": ""
    },
    {
      "name": "",
      "url": ""
    }
  ]
}
```

### 3. Compile the Code

```bash
go build -o jellyfin-podcatcher main.go
```

### 4. Add the Daemon to Systemd
```bash
nano /etc/systemd/system/jellyfin-podcatcher.service
```

```bash
[Unit]
Description=Jellyfin Podcatcher Sync Daemon
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/jellyfin-podcatcher
ExecStart=/opt/jellyfin-podcatcher/jellyfin-podcatcher
Restart=always
RestartSec=10
User=root

# Storage Mapping Specifications
Environment="PODCAST_LIBRARY_DIR=your/jellyfin/library/here"
Environment="PODCAST_RETENTION_DAYS=14"

# Secret Authentication Keys
Environment="JELLYFIN_URL=[http://127.0.0.1:8096](http://127.0.0.1:8096)"
Environment="JELLYFIN_API_KEY=your_secret_jellyfin_admin_api_token"

[Install]
WantedBy=multi-user.target
```

### 5. Start the Service and Verify it is Running
```bash
systemctl daemon-reload
systemctl enable --now jellyfin-podcatcher.service
systemctl status jellyfin-podcatcher
```

### 6. View logs
```bash
cat /opt/jellyfin-podcatcher/activity.log
```

## If Using Docker Rather Than Bare-Metal
Get through setting up your JSON feed and then add the following to your Docker Compose YAML:

```bash
version: '3.8'

services:
  jellyfin-podcatcher:
    build: .
    container_name: jellyfin-podcatcher
    restart: always
    volumes:
      # Mount the config file and the local media storage directory
      - ./config.json:/app/config.json
      - your/jellyfin/podcast/library:/your/jellyfin/podcast/library
    environment:
      - PODCAST_LIBRARY_DIR=/your/jellyfin/podcast/library
      - PODCAST_RETENTION_DAYS=14
      - JELLYFIN_URL=http://127.0.0.1:8096
      - JELLYFIN_API_KEY=your_secret_jellyfin_admin_api_token
```
```bash
docker compose up -d
```

## If Using GUI Rather Than Docker or Bare-Metal
This was designed with absolute ease-of-use in mind.  Run your podcast server from any machine!  Access the GUI from http://<LOCAL_IP>:8080.  Setup your local environment using the GUI.

<img width="805" height="407" alt="image" src="https://github.com/user-attachments/assets/4d47345f-a2ab-41fa-a939-2fb6db00dc7a" />

Search for your podcast title or just a keyword or so.

<img width="738" height="185" alt="image" src="https://github.com/user-attachments/assets/ca7f8c1d-d9ec-498e-8234-d831d037d4b4" />

Hit the "Add" button and the server will automatically add it to your config file, and you start listening right away!

## If Using Windows or macOS Rather Than Linux
### Windows
Native Windows <br/>
Via WSL
### macOS

## Contributing
Anyone is more than welcome to contribute to this project.  Please submit a PR, and we will review it promptly.

## Star History

## Developer Support
If you found this helpful and you would like to support similar development project, please consider buying me a coffee!

buymeacoffee.com/maplewater
