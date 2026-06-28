package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Executor struct {
	baseURL        string
	authHeader     string
	allowedHeaders map[string]bool
	client         *http.Client
}

func (e *Executor) Execute(ctx context.Context, method string, path string, body any, modelHeaders map[string]string) (map[string]any, error) {
	// Validate path is relative (no scheme/host)
	if strings.Contains(path, "://") {
		return nil, fmt.Errorf("path must be relative")
	}

	// Resolve path against baseURL -- skipping lots of error handling..
	base, _ := url.Parse(e.baseURL)
	ref, _ := url.Parse(path)
	target := base.ResolveReference(ref)

	// Ensure target stays within baseURL (prevent /v1evil from matching v1)
	basePrefix := strings.TrimRight(e.baseURL, "/") + "/"
	if !strings.HasPrefix(target.String(), basePrefix) && target.String() != strings.TrimRight(e.baseURL, "/") {
		return nil, fmt.Errorf("path outside base URL")
	}

	// Build request with injected auth
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), target.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %v", err)
	}

	req.Header.Set("Authorization", e.authHeader)
	req.Header.Set("Content-Type", "application/json")

	// Copy only whitelisted headers from the model. Anything not in
	// allowedHeaders is dropped, and Authorization cannot be overridded since
	// it was injected server-side. The model cannot replace it.
	for name, value := range modelHeaders {
		canonical := http.CanonicalHeaderKey(name)
		if canonical == "Authorization" || !e.allowedHeaders[canonical] {
			continue
		}
		req.Header.Set(canonical, value)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("read response ready: %v", err)
	}

	var result any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %v", err)
	}

	return map[string]any{
		"status_code": resp.StatusCode,
		"body":        result,
	}, nil
}
