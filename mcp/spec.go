package mcp

import "strings"

type SpecIndex struct {
	Path       string
	Method     string
	Summary    string
	searchText string
	Tags       []string
	Responses  any
}

type SpecIndicies struct {
	entries []SpecIndex
}

// BuildIndex allows us to index the spec at startup using an inverted index for full-text search.
func BuildIndex(spec map[string]any) *SpecIndicies {
	idx := &SpecIndicies{}

	// Verify the type via type assertion
	paths, _ := spec["paths"].(map[string]any) // paths can be nil. Is that desired? A nil map ranges fine (zero iterations) but panics the moment you write to it.

	for path, pathItem := range paths {
		methods, _ := pathItem.(map[string]any)
		for method, operation := range methods {
			op, _ := operation.(map[string]any)
			summary, _ := op["summary"].(string)
			searchText := strings.ToLower(path + " " + method + " " + summary)
			idx.entries = append(idx.entries, SpecIndex{
				Path:       path,
				Method:     strings.ToUpper(method),
				Summary:    summary,
				searchText: searchText,
				Responses:  op["responses"],
			})
		}
	}
	return idx
}

// Pretty sure the query == `searchText` field from SpecIndex
func (idx *SpecIndicies) Search(query string, limit int) []map[string]any {
	var results []map[string]any
	terms := strings.Fields(strings.ToLower(query))

	for _, entry := range idx.entries {
		score := 0
		for _, term := range terms {
			if strings.Contains(entry.searchText, term) {
				score++
			}
		}

		if score > 0 && len(results) < limit {
			results = append(results, map[string]any{
				"path":      entry.Path,
				"method":    entry.Method,
				"summary":   entry.Summary,
				"responses": entry.Responses,
			})
		}
	}
	return results
}
