# Pre-Download & Offline Viewing

Download entire torrents to disk for instant offline playback with automatic cleanup.

## Quick Start

### Via Web UI
1. Open **Settings → Secondary Settings**
2. Enable **Pre-Download**
3. Set **Download Path** (e.g., `/data/downloads`)
4. Set **Default TTL** (minutes, 0 = no expiry, default 43200 = 30 days)
5. Save settings

### Via API
```bash
# Enable download feature
curl -X POST http://127.0.0.1:8090/settings -H "Content-Type: application/json" -d '{
  "action": "set",
  "sets": {
    "EnableDownload": true,
    "DownloadPath": "/data/downloads",
    "DownloadTTL": 43200
  }
}'

# Add a torrent
curl -X POST http://127.0.0.1:8090/torrents -H "Content-Type: application/json" -d '{
  "action": "add",
  "link": "magnet:?xt=urn:btih:...",
  "save_to_db": true
}'

# Start download (TTL in minutes, 0 = no expiry)
curl -X POST http://127.0.0.1:8090/download -H "Content-Type: application/json" -d '{
  "action": "start",
  "hash": "<infohash>",
  "ttl": 43200
}'

# Check status
curl -X POST http://127.0.0.1:8090/download -H "Content-Type: application/json" -d '{
  "action": "status",
  "hash": "<infohash>"
}'

# List all downloads
curl -X POST http://127.0.0.1:8090/download -H "Content-Type: application/json" -d '{
  "action": "list"
}'

# Cancel download
curl -X POST http://127.0.0.1:8090/download -H "Content-Type: application/json" -d '{
  "action": "cancel",
  "hash": "<infohash>"
}'
```

## Features

- **Instant Playback**: Files served directly from disk (~0.01s response time)
- **Offline Viewing**: Watch without internet connection or peer availability
- **Seeding Support**: Downloaded files are seeded through torrent cache
- **TTL Auto-Cleanup**: Files automatically deleted after configurable period
- **Parallel Download**: ~8 MB/s with readahead optimization
- **Range Requests**: Full video seeking support (HTTP 206)
- **Resume Support**: Disk-backed cache survives server restarts

## How It Works

1. **Download**: Torrent files are downloaded to `{DownloadPath}/{infohash}/`
2. **Cache Marking**: Downloaded pieces marked as `IsDownloaded` to prevent eviction
3. **Offline Serving**: `/play` and `/stream` endpoints check local files first
4. **Seeding**: Pieces remain in cache for seeding to other peers
5. **Cleanup**: Background daemon removes expired downloads hourly

## Settings

| Setting | Description | Default |
|---------|-------------|---------|
| `EnableDownload` | Enable pre-download feature | `false` |
| `DownloadPath` | Directory for downloaded files | `""` |
| `DownloadTTL` | Default TTL in minutes (0 = no expiry) | `43200` (30 days) |

## Storage Structure

```
{DownloadPath}/
  └── {infohash}/
      ├── file1.mkv
      ├── file2.srt
      └── poster.jpg
```

## API Reference

### POST /download

| Action | Parameters | Description |
|--------|------------|-------------|
| `start` | `hash`, `ttl` | Start downloading a torrent |
| `cancel` | `hash` | Cancel download and remove files |
| `status` | `hash` | Get download progress |
| `list` | — | List all downloads |

### Response (status)
```json
{
  "hash": "dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c",
  "path": "/data/downloads/dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c",
  "total_size": 276445467,
  "downloaded": 276478944,
  "file_count": 3,
  "files_done": 3,
  "expiry_date": 1784126143,
  "is_complete": true,
  "status": "completed"
}
```

## Performance

| Metric | Value |
|--------|-------|
| Download Speed | ~8 MB/s |
| Disk Serve Time | ~0.01s |
| Cache Retention | 100% (no eviction) |
| Time to First Byte | < 1ms (from disk) |

## Notes

- Downloaded files are served via `http.ServeContent` (supports Range requests)
- Cache capacity is dynamically expanded during download
- Torrent removal automatically deletes downloaded files
- No database migration required for existing installations
