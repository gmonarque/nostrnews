package rss

import (
	"context"
	"html"
	"regexp"
	"strings"
	"time"

	"nostrnews/config"

	"github.com/mmcdole/gofeed"
)

var (
	paywallKeywords = []string{
		"subscribe to read",
		"subscription required",
		"subscribers only",
		"members only",
		"sign in to continue",
		"log in to read",
		"premium content",
		"exclusive content",
		"unlock this article",
		"for full access",
		"to continue reading",
		"register to read",
	}
	truncationPatterns  = regexp.MustCompile(`(?i)(\.\.\.|…|\[…\]|\[\.\.\.\]|read more|continue reading|read full|see more)$`)
	htmlTagPattern      = regexp.MustCompile(`<[^>]*>`)
	multiSpacePattern   = regexp.MustCompile(`\s{2,}`)
	multiNewlinePattern = regexp.MustCompile(`\n{3,}`)
)

func cleanText(s string) string {

	s = htmlTagPattern.ReplaceAllString(s, " ")

	s = html.UnescapeString(s)
	s = html.UnescapeString(s)

	replacements := map[string]string{
		"&ldquo;":  `"`,
		"&rdquo;":  `"`,
		"&lsquo;":  `'`,
		"&rsquo;":  `'`,
		"&mdash;":  "—",
		"&ndash;":  "–",
		"&hellip;": "…",
		"&bull;":   "•",
		"&nbsp;":   " ",
		"&amp;":    "&",
		"&lt;":     "<",
		"&gt;":     ">",
	}
	for entity, char := range replacements {
		s = strings.ReplaceAll(s, entity, char)
	}

	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "\u200b", "")
	s = strings.ReplaceAll(s, "\ufeff", "")

	s = multiSpacePattern.ReplaceAllString(s, " ")
	s = multiNewlinePattern.ReplaceAllString(s, "\n\n")

	return strings.TrimSpace(s)
}

type Article struct {
	GUID        string
	Title       string
	Link        string
	Description string
	Content     string
	Published   time.Time
	ImageURL    string
	Author      string

	FeedName string
	Country  string
	Language string
	Category string
	Paywall  string
	Tags     []string
}

type Fetcher struct {
	parser *gofeed.Parser
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		parser: gofeed.NewParser(),
	}
}

func (f *Fetcher) Fetch(ctx context.Context, feed config.Feed) ([]Article, error) {
	parsed, err := f.parser.ParseURLWithContext(feed.URL, ctx)
	if err != nil {
		return nil, err
	}

	var articles []Article
	for _, item := range parsed.Items {
		article := Article{
			GUID:        item.GUID,
			Title:       cleanText(item.Title),
			Link:        item.Link,
			Description: cleanText(item.Description),
			Content:     cleanText(item.Content),
			FeedName:    feed.Name,
			Country:     feed.Country,
			Language:    feed.Language,
			Category:    feed.Category,
			Paywall:     detectPaywall(feed.Paywall, item.Content, item.Description),
			Tags:        feed.Tags,
		}

		if article.GUID == "" {
			article.GUID = item.Link
		}

		if item.PublishedParsed != nil {
			article.Published = *item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			article.Published = *item.UpdatedParsed
		} else {
			article.Published = time.Now()
		}

		if item.Image != nil {
			article.ImageURL = item.Image.URL
		} else if len(item.Enclosures) > 0 {
			for _, enc := range item.Enclosures {
				if enc.Type == "image/jpeg" || enc.Type == "image/png" || enc.Type == "image/webp" {
					article.ImageURL = enc.URL
					break
				}
			}
		}

		if item.Author != nil && item.Author.Name != "" {
			article.Author = item.Author.Name
		} else if len(item.Authors) > 0 && item.Authors[0].Name != "" {
			article.Author = item.Authors[0].Name
		}

		articles = append(articles, article)
	}

	return articles, nil
}

func detectPaywall(feedPaywall, content, description string) string {

	if feedPaywall == "hard" {
		return "hard"
	}

	text := content
	if text == "" {
		text = description
	}

	text = htmlTagPattern.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	textLower := strings.ToLower(text)

	for _, keyword := range paywallKeywords {
		if strings.Contains(textLower, keyword) {
			return "hard"
		}
	}

	if truncationPatterns.MatchString(strings.TrimSpace(text)) {

		if feedPaywall == "soft" || feedPaywall == "" {
			return "soft"
		}
	}

	if len(text) < 200 && len(text) > 0 {
		if feedPaywall == "soft" {
			return "soft"
		}

	}

	if len(text) > 500 && feedPaywall != "hard" {
		return "none"
	}

	if feedPaywall == "" {
		return "none"
	}
	return feedPaywall
}
