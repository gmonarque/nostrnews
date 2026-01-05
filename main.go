package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nostrnews/config"
	"nostrnews/nostr"
	"nostrnews/rss"
	"nostrnews/store"
)

const (
	storePath = "published.db"
)

var defaultRelays = []string{
	"wss://relay.damus.io",
	"wss://nos.lol",
	"wss://relay.primal.net",
	"wss://relay.snort.social",
	"wss://nostr.land",
	"wss://nostr-pub.wellorder.net",
	"wss://offchain.pub",
	"wss://relay.nostr.band",
}

func main() {
	// Get private key from environment
	privateKey := os.Getenv("NOSTR_PRIVATE_KEY")
	if privateKey == "" {
		log.Fatal("NOSTR_PRIVATE_KEY environment variable is required")
	}

	// Load embedded feed configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Loaded %d feeds", len(cfg.Feeds))

	// Initialize store
	publishedStore, err := store.New(storePath)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}
	defer publishedStore.Close()

	// Initialize components
	fetcher := rss.NewFetcher()
	publisher, err := nostr.NewPublisher(privateKey, defaultRelays)
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	defer publisher.Close()

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	// Only process articles published after program start (use UTC for consistent comparison)
	// Subtract 5 minutes buffer to catch articles with slightly older timestamps
	startTime := time.Now().UTC().Add(-5 * time.Minute)
	log.Printf("Will only process articles published after %s", startTime.Format(time.RFC3339))

	// Run continuously
	for {
		select {
		case <-ctx.Done():
			return
		default:
			processFeeds(ctx, cfg, fetcher, publisher, publishedStore, startTime)
		}
	}
}

func processFeeds(ctx context.Context, cfg *config.Config, fetcher *rss.Fetcher, publisher *nostr.Publisher, store *store.Store, cutoff time.Time) {
	for _, feed := range cfg.Feeds {
		select {
		case <-ctx.Done():
			return
		default:
		}

		articles, err := fetcher.Fetch(ctx, feed)
		if err != nil {
			// Silently skip fetch errors
			continue
		}

		for _, article := range articles {
			// Skip if already published
			if store.IsPublished(article.GUID) {
				continue
			}

			// Skip untitled articles
			if article.Title == "" || article.Title == "Untitled" {
				store.MarkPublished(article.GUID, time.Now().Unix()) // Mark as processed to skip in future
				continue
			}

			// Skip articles without cover image
			if article.ImageURL == "" {
				store.MarkPublished(article.GUID, time.Now().Unix())
				continue
			}

			// Skip articles without description
			if article.Description == "" && article.Content == "" {
				store.MarkPublished(article.GUID, time.Now().Unix())
				continue
			}

			// Skip articles older than cutoff time (compare in UTC)
			if article.Published.UTC().Before(cutoff) {
				continue
			}

			// Publish to Nostr
			if err := publisher.Publish(ctx, article); err != nil {
				continue
			}

			// Mark as published
			store.MarkPublished(article.GUID, time.Now().Unix())

			// Delay between publications to avoid rate limiting
			time.Sleep(3 * time.Second)
		}
	}
}
