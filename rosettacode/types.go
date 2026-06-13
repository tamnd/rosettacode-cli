package rosettacode

import (
	"strings"
)

// Task is the record emitted for a Rosetta Code programming task.
type Task struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// Language is the record emitted for a programming language on Rosetta Code.
type Language struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// SearchResult is the record emitted for a Rosetta Code search hit.
type SearchResult struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	URL     string `json:"url"`
}

// taskURL builds the canonical Rosetta Code URL for a task title.
// Spaces become underscores per MediaWiki convention.
func taskURL(baseURL, title string) string {
	slug := strings.ReplaceAll(title, " ", "_")
	return baseURL + "/wiki/" + slug
}

// stripWikiMarkup removes common wiki syntax from wikitext to produce
// readable plain text. It handles templates {{...}}, link syntax [[...|...]],
// HTML tags, and common HTML entities.
func stripWikiMarkup(s string) string {
	// Remove templates {{...}} — they may be nested; iterate until stable.
	for {
		next := removeBalanced(s, "{{", "}}")
		if next == s {
			break
		}
		s = next
	}
	// Convert [[target|display]] → display, [[target]] → target.
	s = replaceWikiLinks(s)
	// Strip HTML tags.
	s = stripHTMLTags(s)
	// Unescape common HTML entities.
	s = unescapeEntities(s)
	// Collapse excessive blank lines.
	lines := strings.Split(s, "\n")
	var out []string
	blank := 0
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			blank++
			if blank <= 1 {
				out = append(out, "")
			}
		} else {
			blank = 0
			out = append(out, trimmed)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func removeBalanced(s, open, close string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if strings.HasPrefix(s[i:], open) {
			// Find matching close, skipping nested opens.
			depth := 1
			j := i + len(open)
			for j < len(s) && depth > 0 {
				if strings.HasPrefix(s[j:], open) {
					depth++
					j += len(open)
				} else if strings.HasPrefix(s[j:], close) {
					depth--
					j += len(close)
				} else {
					j++
				}
			}
			// Skip the entire matched block.
			i = j
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

func replaceWikiLinks(s string) string {
	var b strings.Builder
	for {
		start := strings.Index(s, "[[")
		if start == -1 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:start])
		end := strings.Index(s[start:], "]]")
		if end == -1 {
			b.WriteString(s[start:])
			break
		}
		inner := s[start+2 : start+end]
		// [[target|display]] → display; [[target]] → target.
		if pipe := strings.Index(inner, "|"); pipe != -1 {
			b.WriteString(inner[pipe+1:])
		} else {
			b.WriteString(inner)
		}
		s = s[start+end+2:]
	}
	return b.String()
}

func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func unescapeEntities(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&apos;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return s
}
