package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/edgard/opencompat/internal/api"
	"github.com/edgard/opencompat/internal/auth"
	"github.com/edgard/opencompat/internal/config"
	"github.com/edgard/opencompat/internal/upstream"
)

// Maximum request body size (10MB)
const maxRequestBodySize = 10 * 1024 * 1024

// validRoles defines the valid message roles for OpenAI API
var validRoles = map[string]bool{
	"system":    true,
	"user":      true,
	"assistant": true,
	"tool":      true,
}

// generateRequestID generates a unique request ID for the x-request-id header.
func generateRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	// Format as UUID-like string: req_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
	return "req_" + hex.EncodeToString(b)
}

// logIgnoredParameters logs warnings for parameters that are accepted but ignored.
func logIgnoredParameters(req *api.ChatCompletionRequest) {
	var ignored []string

	if req.Temperature != nil {
		ignored = append(ignored, "temperature")
	}
	if req.TopP != nil {
		ignored = append(ignored, "top_p")
	}
	if req.N != nil && *req.N != 1 {
		ignored = append(ignored, "n")
	}
	if req.Stop != nil {
		ignored = append(ignored, "stop")
	}
	if req.MaxTokens != nil {
		ignored = append(ignored, "max_tokens")
	}
	if req.MaxCompletionTokens != nil {
		ignored = append(ignored, "max_completion_tokens")
	}
	if req.PresencePenalty != nil {
		ignored = append(ignored, "presence_penalty")
	}
	if req.FrequencyPenalty != nil {
		ignored = append(ignored, "frequency_penalty")
	}
	if req.LogitBias != nil {
		ignored = append(ignored, "logit_bias")
	}
	if req.Seed != nil {
		ignored = append(ignored, "seed")
	}

	if len(ignored) > 0 {
		log.Printf("Warning: ignoring unsupported parameters: %s", strings.Join(ignored, ", "))
	}
}

// Handlers holds the HTTP handlers and their dependencies.
type Handlers struct {
	store  *auth.Store
	client *upstream.Client
	cfg    *config.Config
}

// NewHandlersWithClient creates a new handlers instance with a shared upstream client.
func NewHandlersWithClient(store *auth.Store, cfg *config.Config, client *upstream.Client) *Handlers {
	return &Handlers{
		store:  store,
		client: client,
		cfg:    cfg,
	}
}

// Health handles GET /health
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteMethodNotAllowed(w)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Models handles GET /v1/models
func (h *Handlers) Models(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteMethodNotAllowed(w)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(api.GetModelsResponse())
}

// ChatCompletions handles POST /v1/chat/completions
func (h *Handlers) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteMethodNotAllowed(w)
		return
	}

	// Generate and set request ID header early
	requestID := generateRequestID()
	w.Header().Set("x-request-id", requestID)

	// Limit request body size to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	// Parse request
	var req api.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			api.WriteBadRequest(w, "Request body too large (max 10MB)")
			return
		}
		api.WriteBadRequest(w, "Invalid JSON: "+err.Error())
		return
	}

	// Log warnings for ignored parameters
	logIgnoredParameters(&req)

	// Validate model
	if req.Model == "" {
		api.WriteBadRequestWithParam(w, "model is required", "model")
		return
	}

	// Normalize model name (handles aliases, provider prefixes, and effort suffixes)
	// Use base model (without effort suffix) for validation and instructions
	normalizedModel, _ := upstream.NormalizeModelNameWithEffort(req.Model)
	if !api.IsModelSupported(normalizedModel) {
		api.WriteModelNotFound(w, req.Model)
		return
	}

	// Validate messages
	if len(req.Messages) == 0 {
		api.WriteBadRequestWithParam(w, "messages is required", "messages")
		return
	}

	// Validate each message
	for i, msg := range req.Messages {
		// Validate role
		if !validRoles[msg.Role] {
			api.WriteBadRequestWithParam(w,
				fmt.Sprintf("Invalid role '%s'. Must be one of: system, user, assistant, tool", msg.Role),
				fmt.Sprintf("messages[%d].role", i))
			return
		}

		// Validate tool messages have tool_call_id
		if msg.Role == "tool" && msg.ToolCallID == "" {
			api.WriteBadRequestWithParam(w,
				"Tool messages must include tool_call_id",
				fmt.Sprintf("messages[%d].tool_call_id", i))
			return
		}
	}

	// Get instructions for the normalized model (without effort suffix)
	instructions, err := h.client.GetInstructions(normalizedModel)
	if err != nil {
		api.WriteServerError(w, "Failed to get model instructions: "+err.Error())
		return
	}

	// Transform to upstream request
	upstreamReq, err := upstream.TransformRequest(&req, instructions, h.cfg)
	if err != nil {
		api.WriteServerError(w, "Failed to transform request: "+err.Error())
		return
	}

	// Send request to ChatGPT
	resp, err := h.client.SendRequest(r.Context(), upstreamReq)
	if err != nil {
		api.WriteServerError(w, "Failed to send request: "+err.Error())
		return
	}
	defer resp.Body.Close()

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// Try to parse as JSON to extract a meaningful error message
		errorMsg := parseUpstreamError(body)
		api.WriteUpstreamError(w, resp.StatusCode, errorMsg)
		return
	}

	// Handle streaming vs non-streaming
	if req.Stream {
		h.handleStreaming(w, resp.Body, &req)
	} else {
		h.handleNonStreaming(w, resp.Body)
	}
}

// parseUpstreamError attempts to extract a meaningful error message from upstream response.
func parseUpstreamError(body []byte) string {
	// Try to parse as JSON error
	var errResp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"` // Alternative format
		Detail  string `json:"detail"`  // Another alternative
	}

	if err := json.Unmarshal(body, &errResp); err == nil {
		if errResp.Error.Message != "" {
			return errResp.Error.Message
		}
		if errResp.Message != "" {
			return errResp.Message
		}
		if errResp.Detail != "" {
			return errResp.Detail
		}
	}

	// If not JSON or no message found, return raw body (truncated if too long)
	bodyStr := string(body)
	if len(bodyStr) > 500 {
		bodyStr = bodyStr[:500] + "..."
	}

	// If it looks like HTML, return a generic message
	if strings.HasPrefix(strings.TrimSpace(bodyStr), "<") {
		return "Upstream server returned an error"
	}

	return bodyStr
}

func (h *Handlers) handleStreaming(w http.ResponseWriter, body io.Reader, req *api.ChatCompletionRequest) {
	sseWriter, err := NewSSEWriter(w)
	if err != nil {
		api.WriteServerError(w, err.Error())
		return
	}

	reader := upstream.NewSSEReader(body)
	state := upstream.NewStreamState()
	state.SetReasoningCompat(h.cfg.ReasoningCompat)

	// Check if client wants usage info
	includeUsage := req.StreamOptions != nil && req.StreamOptions.IncludeUsage

	for {
		event, err := reader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				break
			}
			_ = sseWriter.WriteError("Stream read error: " + err.Error())
			break
		}

		chunks, err := state.ProcessEvent(event)
		if err != nil {
			_ = sseWriter.WriteError("Event processing error: " + err.Error())
			continue
		}

		for _, chunk := range chunks {
			if err := sseWriter.WriteChunk(chunk); err != nil {
				// Client disconnected
				return
			}
		}
	}

	// Check for stream error
	if errMsg := state.GetError(); errMsg != "" {
		_ = sseWriter.WriteError("Upstream error: " + errMsg)
	}

	// Send usage chunk if requested
	if includeUsage {
		if usageChunk := state.GetUsageChunk(); usageChunk != nil {
			_ = sseWriter.WriteChunk(usageChunk)
		}
	}

	_ = sseWriter.WriteDone()
}

func (h *Handlers) handleNonStreaming(w http.ResponseWriter, body io.Reader) {
	reader := upstream.NewSSEReader(body)
	state := upstream.NewStreamState()
	state.SetReasoningCompat(h.cfg.ReasoningCompat)

	// Process all events
	for {
		event, err := reader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				break
			}
			api.WriteServerError(w, "Stream read error: "+err.Error())
			return
		}

		_, err = state.ProcessEvent(event)
		if err != nil {
			api.WriteServerError(w, "Event processing error: "+err.Error())
			return
		}
	}

	// Check for stream error
	if errMsg := state.GetError(); errMsg != "" {
		api.WriteServerError(w, "Upstream error: "+errMsg)
		return
	}

	// Build and send response
	response := state.BuildNonStreamingResponse()
	if response.ID == "" {
		api.WriteServerError(w, "No response received from upstream")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}
