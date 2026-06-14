// Package rubygems is the library behind the rubygems command line:
// the HTTP client, request shaping, and the typed data models for rubygems.org.
//
// The Client paces requests, sets an honest User-Agent, and retries transient
// failures (429 and 5xx). No API key is required.
package rubygems

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"sync"
	"time"
)

// DefaultUserAgent identifies the client to RubyGems.
const DefaultUserAgent = "rubygems/dev (+https://github.com/tamnd/rubygems-cli)"

// Host is the site this client talks to.
const Host = "rubygems.org"

// Config holds all tunable parameters for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://rubygems.org/api/v1",
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Timeout:   15 * time.Second,
		Retries:   3,
	}
}

// Client talks to RubyGems over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Search finds gems matching query. It returns at most limit items (pass 0 for default 10).
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Gem, error) {
	n := limit
	if n <= 0 {
		n = 10
	}
	u := fmt.Sprintf("%s/search.json?query=%s&per_page=%d",
		c.cfg.BaseURL, neturl.QueryEscape(query), n)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var raw []rawGem
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode search: %w", err)
	}
	items := make([]Gem, 0, len(raw))
	for i, r := range raw {
		items = append(items, Gem{
			Rank:        i + 1,
			Name:        r.Name,
			Version:     r.Version,
			Downloads:   r.Downloads,
			Authors:     r.Authors,
			Info:        r.Info,
			HomepageURI: r.HomepageURI,
			SourceURI:   r.SourceCodeURI,
		})
	}
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items, nil
}

// GemInfo fetches info for a single gem by name.
func (c *Client) GemInfo(ctx context.Context, name string) (Gem, error) {
	u := fmt.Sprintf("%s/gems/%s.json", c.cfg.BaseURL, neturl.PathEscape(name))
	body, err := c.get(ctx, u)
	if err != nil {
		return Gem{}, err
	}
	var r rawGem
	if err := json.Unmarshal(body, &r); err != nil {
		return Gem{}, fmt.Errorf("decode gem: %w", err)
	}
	return Gem{
		Rank:        1,
		Name:        r.Name,
		Version:     r.Version,
		Downloads:   r.Downloads,
		Authors:     r.Authors,
		Info:        r.Info,
		HomepageURI: r.HomepageURI,
		SourceURI:   r.SourceCodeURI,
	}, nil
}

// Versions fetches version history for a gem. It returns at most limit items (pass 0 for all).
func (c *Client) Versions(ctx context.Context, name string, limit int) ([]Version, error) {
	u := fmt.Sprintf("%s/versions/%s.json", c.cfg.BaseURL, neturl.PathEscape(name))
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var raw []rawVersion
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode versions: %w", err)
	}
	items := make([]Version, 0, len(raw))
	for i, r := range raw {
		items = append(items, Version{
			Rank:      i + 1,
			Number:    r.Number,
			CreatedAt: r.CreatedAt,
			Downloads: r.DownloadsCount,
			RubyVer:   r.RubyVersion,
		})
	}
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items, nil
}

// ReverseDeps lists gems that depend on the given gem.
// The API returns a flat array of gem name strings; the client converts them to ReverseDep.
// limit clips the result; 0 means return all.
func (c *Client) ReverseDeps(ctx context.Context, name string, limit int) ([]ReverseDep, error) {
	u := fmt.Sprintf("%s/gems/%s/reverse_dependencies.json", c.cfg.BaseURL, neturl.PathEscape(name))
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var names []string
	if err := json.Unmarshal(body, &names); err != nil {
		return nil, fmt.Errorf("decode reverse deps: %w", err)
	}
	items := make([]ReverseDep, 0, len(names))
	for i, n := range names {
		if limit > 0 && i >= limit {
			break
		}
		items = append(items, ReverseDep{
			Rank: i + 1,
			Name: n,
			URL:  "https://rubygems.org/gems/" + n,
		})
	}
	return items, nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	return b, err != nil, err
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	return min(time.Duration(attempt)*500*time.Millisecond, 5*time.Second)
}
