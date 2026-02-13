package nostr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"nostrnews/rss"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

type Publisher struct {
	secretKey     string
	pubKey        string
	relays        []string
	pool          *nostr.SimplePool
	relayCooldown map[string]time.Time
	cooldownMu    sync.RWMutex
}

const rateLimitCooldown = 60 * time.Second

func NewPublisher(privateKey string, relays []string) (*Publisher, error) {
	privateKeyHex, err := normalizePrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	pubKey, err := nostr.GetPublicKey(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	return &Publisher{
		secretKey:     privateKeyHex,
		pubKey:        pubKey,
		relays:        relays,
		pool:          nostr.NewSimplePool(context.Background()),
		relayCooldown: make(map[string]time.Time),
	}, nil
}

func normalizePrivateKey(privateKey string) (string, error) {
	key := strings.TrimSpace(privateKey)
	if key == "" {
		return "", fmt.Errorf("private key is empty")
	}

	if strings.HasPrefix(key, "nsec1") {
		prefix, decodedValue, err := nip19.Decode(key)
		if err != nil {
			return "", fmt.Errorf("failed to decode nsec key: %w", err)
		}
		if prefix != "nsec" {
			return "", fmt.Errorf("expected nsec key, got %s", prefix)
		}

		decodedKey, ok := decodedValue.(string)
		if !ok {
			return "", fmt.Errorf("unexpected decoded key type %T", decodedValue)
		}
		key = decodedKey
	}

	if len(key) != 64 {
		return "", fmt.Errorf("expected 64 hex chars, got %d", len(key))
	}
	if _, err := hex.DecodeString(key); err != nil {
		return "", err
	}

	return key, nil
}

func (p *Publisher) Publish(ctx context.Context, article rss.Article) error {

	archiveURL := GetArchiveURL(article.Link)

	content := p.buildContent(article, archiveURL)

	dTag := p.hashGUID(article.GUID)

	tags := nostr.Tags{
		{"d", dTag},
		{"title", article.Title},
		{"published_at", strconv.FormatInt(article.Published.Unix(), 10)},
		{"r", article.Link},
	}

	if article.Description != "" {
		summary := article.Description
		if len(summary) > 500 {
			summary = summary[:497] + "..."
		}
		tags = append(tags, nostr.Tag{"summary", summary})
	}

	if article.ImageURL != "" {
		tags = append(tags, nostr.Tag{"image", article.ImageURL})
	}

	if article.Country != "" {
		tags = append(tags, nostr.Tag{"country", article.Country})
	}
	if article.Language != "" {
		tags = append(tags, nostr.Tag{"language", article.Language})
	}
	if article.Category != "" {
		tags = append(tags, nostr.Tag{"category", article.Category})
	}
	if article.Paywall != "" {
		tags = append(tags, nostr.Tag{"paywall", article.Paywall})
	}

	tags = append(tags, nostr.Tag{"source", article.FeedName})

	if article.Author != "" {
		tags = append(tags, nostr.Tag{"author", article.Author})
	}

	for _, t := range article.Tags {
		tags = append(tags, nostr.Tag{"t", t})
	}

	tags = append(tags, nostr.Tag{"archive", archiveURL})

	ev := nostr.Event{
		PubKey:    p.pubKey,
		CreatedAt: nostr.Now(),
		Kind:      30023,
		Tags:      tags,
		Content:   content,
	}

	if err := ev.Sign(p.secretKey); err != nil {
		return fmt.Errorf("failed to sign event: %w", err)
	}

	successCount := 0
	for _, relayURL := range p.relays {

		if p.isInCooldown(relayURL) {
			continue
		}

		relay, err := p.pool.EnsureRelay(relayURL)
		if err != nil {

			continue
		}

		err = relay.Publish(ctx, ev)
		if err != nil {
			errMsg := err.Error()

			if strings.Contains(errMsg, "rate-limited") || strings.Contains(errMsg, "rate limit") {
				p.setCooldown(relayURL)
				continue
			}

			if strings.Contains(errMsg, "replaced") {
				successCount++
				continue
			}

			continue
		}
		successCount++
	}

	if successCount == 0 {
		return fmt.Errorf("failed to publish to any relay")
	}

	log.Printf("[%d relays] %s", successCount, article.Title)
	return nil
}

func (p *Publisher) isInCooldown(relayURL string) bool {
	p.cooldownMu.RLock()
	defer p.cooldownMu.RUnlock()
	cooldownUntil, exists := p.relayCooldown[relayURL]
	if !exists {
		return false
	}
	return time.Now().Before(cooldownUntil)
}

func (p *Publisher) setCooldown(relayURL string) {
	p.cooldownMu.Lock()
	defer p.cooldownMu.Unlock()
	p.relayCooldown[relayURL] = time.Now().Add(rateLimitCooldown)
}

func (p *Publisher) buildContent(article rss.Article, archiveURL string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("**Source:** [%s](%s) | [archive](%s)\n\n---\n\n", article.FeedName, article.Link, archiveURL))

	body := article.Content
	if body == "" {
		body = article.Description
	}

	sb.WriteString(body)

	return sb.String()
}

func (p *Publisher) hashGUID(guid string) string {
	h := sha256.Sum256([]byte(guid))
	return hex.EncodeToString(h[:16])
}

func (p *Publisher) Close() {

}

func GetArchiveURL(articleURL string) string {
	return "https://archive.is/newest/" + articleURL
}
