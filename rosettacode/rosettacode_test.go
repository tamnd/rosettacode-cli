package rosettacode_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/rosettacode-cli/rosettacode"
)

func newTestClient(baseURL string) *rosettacode.Client {
	cfg := rosettacode.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.Rate = 0
	return rosettacode.NewClient(cfg)
}

func TestGetSendsUserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		// Return a valid category members JSON so Tasks() succeeds.
		resp := map[string]any{
			"query": map[string]any{
				"categorymembers": []map[string]any{},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	// Use Tasks as a proxy to exercise the HTTP client.
	_, err := c.Tasks(context.Background(), 1, "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		resp := map[string]any{
			"query": map[string]any{
				"random": []map[string]any{
					{"title": "Some Task"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := rosettacode.DefaultConfig()
	cfg.Rate = 0
	cfg.Retries = 5
	cfg.BaseURL = srv.URL
	c := rosettacode.NewClient(cfg)

	start := time.Now()
	tasks, err := c.Random(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Errorf("got %d tasks after retries, want 1", len(tasks))
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestTasks(t *testing.T) {
	resp := map[string]any{
		"query": map[string]any{
			"categorymembers": []map[string]any{
				{"title": "Fibonacci sequence"},
				{"title": "Hello world/Text"},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	tasks, err := c.Tasks(context.Background(), 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
	if tasks[0].Title != "Fibonacci sequence" {
		t.Errorf("title = %q", tasks[0].Title)
	}
	if tasks[0].URL == "" {
		t.Error("URL is empty")
	}
}

func TestTasksWithLangFilter(t *testing.T) {
	var gotCategory string
	resp := map[string]any{
		"query": map[string]any{
			"categorymembers": []map[string]any{
				{"title": "Fibonacci sequence"},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCategory = r.URL.Query().Get("cmtitle")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Tasks(context.Background(), 10, "Go")
	if err != nil {
		t.Fatal(err)
	}
	if gotCategory != "Category:Go" {
		t.Errorf("cmtitle = %q, want Category:Go", gotCategory)
	}
}

func TestSearch(t *testing.T) {
	resp := map[string]any{
		"query": map[string]any{
			"search": []map[string]any{
				{"title": "Fibonacci sequence", "snippet": "compute fib"},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	results, err := c.Search(context.Background(), "fibonacci", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Title != "Fibonacci sequence" {
		t.Errorf("title = %q", results[0].Title)
	}
}

func TestRandom(t *testing.T) {
	resp := map[string]any{
		"query": map[string]any{
			"random": []map[string]any{
				{"title": "Some Task"},
				{"title": "Another Task"},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	tasks, err := c.Random(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
}

func TestTaskFetch(t *testing.T) {
	resp := map[string]any{
		"query": map[string]any{
			"pages": map[string]any{
				"12345": map[string]any{
					"title": "Fibonacci sequence",
					"revisions": []map[string]any{
						{"*": "{{task|Compute fibonacci numbers}}\n[[Go]] solution here."},
					},
				},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	task, err := c.Task(context.Background(), "Fibonacci sequence")
	if err != nil {
		t.Fatal(err)
	}
	if task.Title != "Fibonacci sequence" {
		t.Errorf("title = %q", task.Title)
	}
	if task.Snippet == "" {
		t.Error("Snippet is empty")
	}
	// Template {{...}} should be stripped.
	if strings.Contains(task.Snippet, "{{") {
		t.Errorf("Snippet still has template: %q", task.Snippet)
	}
}

func TestLangs(t *testing.T) {
	resp := map[string]any{
		"query": map[string]any{
			"categorymembers": []map[string]any{
				{"title": "Category:Go"},
				{"title": "Category:Python"},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	langs, err := c.Langs(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(langs) != 2 {
		t.Fatalf("got %d langs, want 2", len(langs))
	}
	if langs[0].Name != "Go" {
		t.Errorf("name = %q, want Go", langs[0].Name)
	}
}
