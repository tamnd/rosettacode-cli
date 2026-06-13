// Package rosettacode is the library behind the rc command line:
// the HTTP client, request shaping, and the typed data models for Rosetta Code.
//
// It speaks to the MediaWiki API at https://rosettacode.org/w/api.php.
// The Client paces requests, retries 429/5xx errors, and returns typed records.
package rosettacode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL   = "https://rosettacode.org"
	defaultAPIPath   = "/w/api.php"
	DefaultUserAgent = "rc/dev (+https://github.com/tamnd/rosettacode-cli)"
)

// Config holds constructor parameters for Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   defaultBaseURL,
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Retries:   5,
		Timeout:   30 * time.Second,
	}
}

// Client talks to the Rosetta Code MediaWiki API.
type Client struct {
	http      *http.Client
	userAgent string
	baseURL   string
	rate      time.Duration
	retries   int
	mu        sync.Mutex
	last      time.Time
}

// NewClient returns a Client built from cfg.
func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = DefaultUserAgent
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &Client{
		http:      &http.Client{Timeout: cfg.Timeout},
		userAgent: cfg.UserAgent,
		baseURL:   cfg.BaseURL,
		rate:      cfg.Rate,
		retries:   cfg.Retries,
	}
}

// apiURL builds a full URL to the MediaWiki API endpoint.
func (c *Client) apiURL() string {
	return c.baseURL + defaultAPIPath
}

// get fetches a URL with pacing and retries, returning the body.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

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
	b, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// getJSON fetches rawURL and decodes JSON into v.
func (c *Client) getJSON(ctx context.Context, rawURL string, v any) error {
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode %s: %w", rawURL, err)
	}
	return nil
}

// ─── wire types ──────────────────────────────────────────────────────────────

type mwCategoryMembersResp struct {
	Continue *struct {
		CMContinue string `json:"cmcontinue"`
	} `json:"continue"`
	Query struct {
		CategoryMembers []struct {
			Title string `json:"title"`
		} `json:"categorymembers"`
	} `json:"query"`
}

type mwSearchResp struct {
	Query struct {
		Search []struct {
			Title   string `json:"title"`
			Snippet string `json:"snippet"`
		} `json:"search"`
	} `json:"query"`
}

type mwRandomResp struct {
	Query struct {
		Random []struct {
			Title string `json:"title"`
		} `json:"random"`
	} `json:"query"`
}

type mwRevisionsResp struct {
	Query struct {
		Pages map[string]struct {
			Title    string `json:"title"`
			Missing  string `json:"missing,omitempty"`
			Revisions []struct {
				Content string `json:"*"`
			} `json:"revisions"`
		} `json:"pages"`
	} `json:"query"`
}

// ─── API methods ──────────────────────────────────────────────────────────────

// Tasks lists programming tasks from Category:Programming_Tasks.
// If lang is non-empty, it lists from "Category:<lang>" instead.
// Results are limited to at most limit items (0 = no limit).
func (c *Client) Tasks(ctx context.Context, limit int, lang string) ([]Task, error) {
	category := "Category:Programming_Tasks"
	if lang != "" {
		category = "Category:" + lang
	}
	return c.categoryMembers(ctx, category, limit)
}

// Langs lists languages from Category:Solutions_by_Programming_Language.
func (c *Client) Langs(ctx context.Context, limit int) ([]Language, error) {
	members, err := c.categoryMembers(ctx, "Category:Solutions_by_Programming_Language", limit)
	if err != nil {
		return nil, err
	}
	langs := make([]Language, len(members))
	for i, m := range members {
		name := strings.TrimPrefix(m.Title, "Category:")
		langs[i] = Language{
			Name: name,
			URL:  taskURL(c.baseURL, m.Title),
		}
	}
	return langs, nil
}

// categoryMembers fetches members of a MediaWiki category, up to limit (0=all).
func (c *Client) categoryMembers(ctx context.Context, category string, limit int) ([]Task, error) {
	var results []Task
	var cont string

	pageSize := 50
	if limit > 0 && limit < pageSize {
		pageSize = limit
	}

	for {
		params := url.Values{}
		params.Set("action", "query")
		params.Set("list", "categorymembers")
		params.Set("cmtitle", category)
		params.Set("cmlimit", fmt.Sprintf("%d", pageSize))
		params.Set("format", "json")
		if cont != "" {
			params.Set("cmcontinue", cont)
		}

		rawURL := c.apiURL() + "?" + params.Encode()
		var resp mwCategoryMembersResp
		if err := c.getJSON(ctx, rawURL, &resp); err != nil {
			return results, err
		}

		for _, m := range resp.Query.CategoryMembers {
			results = append(results, Task{
				Title: m.Title,
				URL:   taskURL(c.baseURL, m.Title),
			})
			if limit > 0 && len(results) >= limit {
				return results, nil
			}
		}

		if resp.Continue == nil || resp.Continue.CMContinue == "" {
			break
		}
		cont = resp.Continue.CMContinue
	}
	return results, nil
}

// Task fetches the wikitext content of a task page and returns a Task with
// cleaned-up Snippet.
func (c *Client) Task(ctx context.Context, title string) (Task, error) {
	params := url.Values{}
	params.Set("action", "query")
	params.Set("prop", "revisions")
	params.Set("rvprop", "content")
	params.Set("titles", title)
	params.Set("format", "json")

	rawURL := c.apiURL() + "?" + params.Encode()
	var resp mwRevisionsResp
	if err := c.getJSON(ctx, rawURL, &resp); err != nil {
		return Task{}, err
	}

	for _, page := range resp.Query.Pages {
		if page.Missing != "" {
			return Task{}, fmt.Errorf("task not found: %s", title)
		}
		content := ""
		if len(page.Revisions) > 0 {
			content = page.Revisions[0].Content
		}
		return Task{
			Title:   page.Title,
			URL:     taskURL(c.baseURL, page.Title),
			Snippet: stripWikiMarkup(content),
		}, nil
	}
	return Task{}, fmt.Errorf("task not found: %s", title)
}

// Search searches Rosetta Code via the MediaWiki search API.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	params := url.Values{}
	params.Set("action", "query")
	params.Set("list", "search")
	params.Set("srsearch", query)
	params.Set("srlimit", fmt.Sprintf("%d", limit))
	params.Set("format", "json")

	rawURL := c.apiURL() + "?" + params.Encode()
	var resp mwSearchResp
	if err := c.getJSON(ctx, rawURL, &resp); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(resp.Query.Search))
	for _, s := range resp.Query.Search {
		results = append(results, SearchResult{
			Title:   s.Title,
			Snippet: stripWikiMarkup(s.Snippet),
			URL:     taskURL(c.baseURL, s.Title),
		})
	}
	return results, nil
}

// Random returns n random Rosetta Code pages.
func (c *Client) Random(ctx context.Context, limit int) ([]Task, error) {
	if limit <= 0 {
		limit = 5
	}
	params := url.Values{}
	params.Set("action", "query")
	params.Set("list", "random")
	params.Set("rnnamespace", "0")
	params.Set("rnlimit", fmt.Sprintf("%d", limit))
	params.Set("format", "json")

	rawURL := c.apiURL() + "?" + params.Encode()
	var resp mwRandomResp
	if err := c.getJSON(ctx, rawURL, &resp); err != nil {
		return nil, err
	}

	tasks := make([]Task, 0, len(resp.Query.Random))
	for _, r := range resp.Query.Random {
		tasks = append(tasks, Task{
			Title: r.Title,
			URL:   taskURL(c.baseURL, r.Title),
		})
	}
	return tasks, nil
}
