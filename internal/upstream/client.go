package upstream

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/edgard/opencompat/internal/auth"
	"github.com/edgard/opencompat/internal/config"
)

// HTTP client configuration
const (
	HTTPTimeout = 5 * time.Minute // Long timeout for streaming responses

	// Codex CLI client identification - matches official client
	DefaultOriginator = "codex_cli_rs"
	CodexVersion      = "0.77.0" // Matches latest Codex CLI release
)

// userAgent is computed once at package initialization
var userAgent string

func init() {
	userAgent = buildUserAgent()
}

// buildUserAgent constructs a User-Agent string matching the Codex CLI format:
// codex_cli_rs/{version} ({OS} {version}; {arch}) {terminal}
func buildUserAgent() string {
	osInfo := getOSInfo()
	arch := getArchitecture()
	terminal := getTerminalInfo()
	return fmt.Sprintf("%s/%s (%s; %s) %s",
		DefaultOriginator, CodexVersion, osInfo, arch, terminal)
}

// getArchitecture returns the architecture string matching Codex CLI format
func getArchitecture() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}

// getTerminalInfo detects the terminal from environment variables
func getTerminalInfo() string {
	program := os.Getenv("TERM_PROGRAM")
	version := os.Getenv("TERM_PROGRAM_VERSION")
	term := os.Getenv("TERM")

	var result string
	if program != "" {
		if version != "" {
			result = program + "/" + version
		} else {
			result = program
		}
	} else if term != "" {
		result = term
	} else {
		result = "unknown"
	}
	return sanitizeHeaderValue(result)
}

// sanitizeHeaderValue removes invalid header characters, replacing them with underscores
func sanitizeHeaderValue(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '/' {
			b.WriteRune(c)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// Client handles communication with the ChatGPT backend API.
type Client struct {
	httpClient     *http.Client
	store          *auth.Store
	cache          *InstructionsCache
	cfg            *config.Config
	cancelRefresh  context.CancelFunc
	refreshContext context.Context
}

// NewClient creates a new upstream client.
func NewClient(store *auth.Store, cfg *config.Config) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: HTTPTimeout,
		},
		store: store,
		cache: NewInstructionsCache(),
		cfg:   cfg,
	}
}

// PrefetchInstructions fetches all instruction files on startup.
// This should be called before starting the HTTP server.
// Returns error if instructions cannot be loaded (no cache AND GitHub down).
func (c *Client) PrefetchInstructions() error {
	return c.cache.Prefetch()
}

// StartBackgroundRefresh starts the background refresh goroutine.
// Call this after PrefetchInstructions succeeds.
func (c *Client) StartBackgroundRefresh() {
	if c.cancelRefresh != nil {
		// Already running
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.refreshContext = ctx
	c.cancelRefresh = cancel

	interval := time.Duration(c.cfg.InstructionsRefresh) * time.Minute
	c.cache.StartBackgroundRefresh(ctx, interval)
}

// Close stops the background refresh and cleans up resources.
func (c *Client) Close() {
	if c.cancelRefresh != nil {
		c.cancelRefresh()
		c.cancelRefresh = nil
	}
}

// SendRequest sends a chat completion request to ChatGPT and returns a reader for SSE events.
func (c *Client) SendRequest(ctx context.Context, req *ResponsesRequest) (*http.Response, error) {
	// Get auth bundle
	bundle, err := c.store.GetBundle()
	if err != nil {
		return nil, fmt.Errorf("auth error: %w", err)
	}

	// Marshal request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", config.ChatGPTResponsesURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Set headers to mimic Codex CLI client exactly
	httpReq.Header.Set("Authorization", "Bearer "+bundle.AccessToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("User-Agent", userAgent)
	httpReq.Header.Set("OpenAI-Beta", "responses=experimental")
	httpReq.Header.Set("originator", DefaultOriginator)

	if bundle.AccountID != "" {
		httpReq.Header.Set("ChatGPT-Account-ID", bundle.AccountID)
	}

	if req.PromptCacheKey != "" {
		httpReq.Header.Set("session_id", req.PromptCacheKey)
		httpReq.Header.Set("conversation_id", req.PromptCacheKey)
	}

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// GetInstructions fetches instructions for a model.
func (c *Client) GetInstructions(modelID string) (string, error) {
	return c.cache.Get(modelID)
}

// SSEReader reads SSE events from an HTTP response.
type SSEReader struct {
	reader *bufio.Reader
	done   bool
}

// NewSSEReader creates a new SSE reader.
func NewSSEReader(r io.Reader) *SSEReader {
	return &SSEReader{
		reader: bufio.NewReader(r),
	}
}

// ReadEvent reads the next SSE event.
func (r *SSEReader) ReadEvent() (*SSEEvent, error) {
	if r.done {
		return nil, io.EOF
	}

	var event SSEEvent
	var dataLines []string

	for {
		line, err := r.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				r.done = true
				if len(dataLines) > 0 {
					// Process any remaining data
					break
				}
			}
			return nil, err
		}

		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		// Empty line signals end of event
		if line == "" {
			if event.Event != "" || len(dataLines) > 0 {
				break
			}
			continue
		}

		// Parse field
		if strings.HasPrefix(line, "event:") {
			event.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)
			if data == "[DONE]" {
				r.done = true
				return nil, io.EOF
			}
			dataLines = append(dataLines, data)
		} else if strings.HasPrefix(line, "id:") {
			event.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		} else if strings.HasPrefix(line, "retry:") {
			if v, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "retry:"))); err == nil {
				event.Retry = v
			}
		}
		// Ignore comments (lines starting with :)
	}

	// Combine data lines
	if len(dataLines) > 0 {
		event.Data = json.RawMessage(strings.Join(dataLines, "\n"))
	}

	return &event, nil
}
