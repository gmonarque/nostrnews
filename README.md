# Nostr News

A simple & minimalist decentralized news aggregator with two components: a Go backend that publishes RSS feeds to Nostr, and a static website that displays the articles.

## Architecture

```
RSS Feeds → [Go Publisher] → Nostr Relays → [Static Website] → Users
```

## Components

### Go Publisher

Continuously fetches articles from configured RSS feeds and publishes them as long-form content (NIP-23, kind 30023) to Nostr relays.

**Features:**
- Configurable RSS feed sources with metadata (country, category, tags, paywall status)
- SQLite database to track published articles and avoid duplicates
- Automatic archive.is links for paywalled content

**Requirements:**
- Go 1.23+
- `NOSTR_PRIVATE_KEY` environment variable (hex-encoded nsec, or NIP-19 nsec)

**Usage:**
```bash
export NOSTR_PRIVATE_KEY="your-hex-private-key"
go run .
```

**Configuration:**

Feeds are configured in `config/feeds.json`:
```json
{
  "feeds": [
    {
      "url": "https://example.com/rss",
      "name": "Example News",
      "country": "us",
      "language": "en",
      "category": "technology",
      "paywall": "none",
      "tags": ["tech", "news"]
    }
  ]
}
```

### Static Website

A client-side web app that fetches and displays articles from Nostr relays.

**Features:**
- Real-time news feed from Nostr relays
- Filter by country, category, tag, or source
- Infinite scroll with automatic loading
- Auto-refresh for new articles (buggy)
- Modal view for full article content
- Archive links for paywalled content
- Persistent filter preferences via localStorage

**Tech Stack:**
- Vanilla JavaScript
- [nostr-tools](https://github.com/nbd-wtf/nostr-tools)
- Static HTML/CSS

## Deployment

Run the Go program on any server or locally:
```bash
go build -o nostrnews
./nostrnews
```

## License

MIT
