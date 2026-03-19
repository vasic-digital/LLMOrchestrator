# API Reference

## Package `agent`

### Interfaces

#### `Agent`
Core interface for all CLI agents.

| Method | Signature | Description |
|--------|-----------|-------------|
| `ID` | `() string` | Unique instance identifier |
| `Name` | `() string` | Agent type name |
| `Start` | `(ctx context.Context) error` | Launch agent process |
| `Stop` | `(ctx context.Context) error` | Graceful shutdown |
| `IsRunning` | `() bool` | Process active status |
| `Health` | `(ctx context.Context) HealthStatus` | Health check |
| `Send` | `(ctx context.Context, prompt string) (Response, error)` | Send prompt, get response |
| `SendStream` | `(ctx context.Context, prompt string) (<-chan StreamChunk, error)` | Streaming response |
| `SendWithAttachments` | `(ctx context.Context, prompt string, attachments []Attachment) (Response, error)` | Send with files |
| `OutputDir` | `() string` | Artifact output directory |
| `Capabilities` | `() AgentCapabilities` | Agent capabilities |
| `SupportsVision` | `() bool` | Vision support |
| `ModelInfo` | `() ModelInfo` | Model information |

#### `AgentPool`
Thread-safe pool with capability matching.

| Method | Signature | Description |
|--------|-----------|-------------|
| `Register` | `(agent Agent) error` | Add agent to pool |
| `Acquire` | `(ctx context.Context, requirements AgentRequirements) (Agent, error)` | Get matching agent (blocks) |
| `Release` | `(agent Agent)` | Return agent to pool |
| `Available` | `() []Agent` | List available agents |
| `HealthCheck` | `(ctx context.Context) []HealthStatus` | Check all agents |
| `Shutdown` | `(ctx context.Context) error` | Stop all agents |

### Types

- `Response` - Parsed result from Send()
- `StreamChunk` - Individual streaming chunk
- `Attachment` - File attachment
- `AgentCapabilities` - Agent feature flags
- `AgentRequirements` - Caller requirements
- `HealthStatus` - Health check result
- `Action` - Structured action (click, type, scroll, etc.)
- `ParsedResponse` - Fully parsed response
- `Issue` - Detected problem
- `ModelInfo` - LLM model information

### Functions

- `NewPool() AgentPool` - Create thread-safe agent pool
- `NewCircuitBreaker() *CircuitBreaker` - Create circuit breaker (default config)
- `NewCircuitBreakerWithConfig(threshold int, timeout time.Duration) *CircuitBreaker` - Custom config
- `NewHealthMonitor() *HealthMonitor` - Create health monitor

## Package `adapter`

### Constructors

| Function | Description |
|----------|-------------|
| `NewOpenCodeAgent(id string, config AdapterConfig) *OpenCodeAgent` | OpenCode adapter |
| `NewClaudeCodeAgent(id string, config AdapterConfig) *ClaudeCodeAgent` | Claude Code adapter |
| `NewGeminiAgent(id string, config AdapterConfig) *GeminiAgent` | Gemini adapter |
| `NewJunieAgent(id string, config AdapterConfig) *JunieAgent` | Junie adapter |
| `NewQwenCodeAgent(id string, config AdapterConfig) *QwenCodeAgent` | Qwen Code adapter |
| `NewBaseAdapter(id, name string, config AdapterConfig, caps AgentCapabilities, modelInfo ModelInfo) *BaseAdapter` | Base adapter |

### `AdapterConfig`

```go
type AdapterConfig struct {
    BinaryPath string
    Args       []string
    Env        []string
    WorkDir    string
    OutputDir  string
    Timeout    time.Duration
    MaxRetries int
}
```

## Package `protocol`

### `PipeTransport`

| Method | Description |
|--------|-------------|
| `NewPipeTransport(reader io.Reader, writer io.Writer) *PipeTransport` | Create transport |
| `Send(ctx, msg PipeMessage) error` | Write JSON-line |
| `Receive(ctx) (PipeMessage, error)` | Read JSON-line |
| `SendPrompt(ctx, requestID, content, imagePath string) error` | Convenience prompt |
| `SendShutdown(ctx) error` | Send shutdown signal |
| `Close() error` | Mark closed |

### `FileTransport`

| Method | Description |
|--------|-------------|
| `NewFileTransport(sessionDir string) (*FileTransport, error)` | Create with inbox/outbox/shared |
| `WriteToInbox(msg FileMessage) error` | Write to inbox |
| `WriteToOutbox(msg FileMessage) error` | Write to outbox |
| `ReadFromInbox() ([]FileMessage, error)` | Read inbox messages |
| `ReadFromOutbox() ([]FileMessage, error)` | Read outbox messages |
| `WriteSharedFile(name string, data []byte) error` | Write shared artifact |
| `ReadSharedFile(name string) ([]byte, error)` | Read shared artifact |
| `Cleanup() error` | Remove session directory |

## Package `parser`

### `ResponseParser` Interface

| Method | Description |
|--------|-------------|
| `Parse(raw string) (ParsedResponse, error)` | Full parse |
| `ExtractJSON(raw string) (map[string]any, error)` | Extract JSON |
| `ExtractActions(raw string) ([]Action, error)` | Extract actions |
| `ExtractIssues(raw string) ([]Issue, error)` | Extract issues |

### Functions

- `NewParser() ResponseParser` - Create default parser

## Package `config`

### Functions

| Function | Description |
|----------|-------------|
| `DefaultConfig() *Config` | Sane defaults |
| `LoadFromEnv(path string) (*Config, error)` | Load from .env file |
| `LoadFromEnvironment() *Config` | Load from OS env |
| `MaskAPIKey(key string) string` | Mask key for logging |

### `Config` Methods

| Method | Description |
|--------|-------------|
| `AgentBinaryPath(name string) (string, error)` | Resolve binary path |
| `IsAgentEnabled(name string) bool` | Check if enabled |
| `SessionDir(sessionID string) string` | Get session directory |
| `Validate() error` | Validate configuration |
