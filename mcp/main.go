package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"` // TODO: What advantages does jsonschema provide over builtin json?
	ID      any             `json:"id"`      // TODO: why any?
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"` // What do params contain? How is it structured?
}

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"` // TODO: Look into GenerateSchema and Anthropics type to see how this correlates. This `any` in the signature is because ToolHandler returns any.
}

// Why return (any, error) rather than (string, error)?
// Similar to the Function field of ToolDefinition, which takes json.RawMessage as input and (string, error) as return values.
// anthropic.Message.Content contains the sessionID & input which is json.RawMessage.
type ToolHandler func(ctx context.Context, sessionID string, input json.RawMessage) (any, error)

type Server struct {
	tools    []Tool
	dispatch map[string]ToolHandler // key is the tool name and value is the tool function.
	mu       sync.Mutex
}

func NewServer() *Server {
	return &Server{dispatch: make(map[string]ToolHandler)}
}

func (s *Server) Register(tool Tool, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools = append(s.tools, tool)
	s.dispatch[tool.Name] = handler
}

// Tell me more about http.ResponseWriter and http.Request
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read the content of the request
	body, err := io.ReadAll(
		io.LimitReader(r.Body, 1<<20), // 100,000,000,000,000,000,000
	)

	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}

	var req Request

	// Unmarshal the body of the request into req
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("failed to unmarshal request: %v", err), http.StatusBadRequest)
		return
	}

	sessionID := r.Header.Get("X-Session-ID")
	var result any

	switch req.Method {
	case "tools/list": // Provide a list of tools
		s.mu.Lock()
		result = map[string]any{"tools": s.tools} // Looks like tools are registered in the Register function.
		s.mu.Unlock()
	case "tools/call":
		var p struct {
			Name  string          `json:"name"`
			Input json.RawMessage `json:"arguments"` // Why Input / Arguments while in the request it's params? Design decision..
		}

		if err := json.Unmarshal(req.Params, &p); err != nil {
			http.Error(w, fmt.Sprintf("failed to unmarshal params: %v", err), http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		handler := s.dispatch[p.Name] // In Register method, handler == ToolHandler. Same here.
		s.mu.Unlock()                 // Why do we only unlock after setting the handler name?

		if handler != nil {
			var handlerErr error
			result, handlerErr = handler(r.Context(), sessionID, p.Input)
			if handlerErr != nil { // Why check if handlerErr is nil? Haven't we already confirmed it is?
				http.Error(w, fmt.Sprintf("tool call failed: %v", handlerErr), http.StatusInternalServerError)
				return
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result":  result,
	})
}
