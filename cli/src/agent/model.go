package agent

// UseOnly controls which targets are built from a source file.
type UseOnly string

const (
	UseOnlyBoth     UseOnly = ""
	UseOnlyClaude   UseOnly = "claude"
	UseOnlyOpencode UseOnly = "opencode"
)

// Source is the parsed representation of a single-source agent markdown file.
type Source struct {
	// Core shared fields
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Model       string   `yaml:"model,omitempty"`
	Tools       []string `yaml:"tools,omitempty"`
	Color       string   `yaml:"color,omitempty"`
	UseOnly     UseOnly  `yaml:"useonly,omitempty"`

	// Target-specific override blocks (raw maps for pass-through)
	Claude    map[string]interface{} `yaml:"claude,omitempty"`
	Opencode  map[string]interface{} `yaml:"opencode,omitempty"`

	// Body is the markdown content after the frontmatter delimiter.
	Body string `yaml:"-"`
}

// ClaudeOut is the rendered frontmatter for a Claude Code agent file.
type ClaudeOut struct {
	Name            string      `yaml:"name"`
	Description     string      `yaml:"description"`
	Model           string      `yaml:"model,omitempty"`
	Tools           string      `yaml:"tools,omitempty"` // CSV
	Color           string      `yaml:"color,omitempty"`
	PermissionMode  string      `yaml:"permissionMode,omitempty"`
	DisallowedTools string      `yaml:"disallowedTools,omitempty"` // CSV
	MaxTurns        interface{} `yaml:"maxTurns,omitempty"`
	Skills          interface{} `yaml:"skills,omitempty"`
	McpServers      interface{} `yaml:"mcpServers,omitempty"`
	Hooks           interface{} `yaml:"hooks,omitempty"`
	Memory          string      `yaml:"memory,omitempty"`
	Background      interface{} `yaml:"background,omitempty"`
	Effort          string      `yaml:"effort,omitempty"`
	Isolation       string      `yaml:"isolation,omitempty"`
	InitialPrompt   string      `yaml:"initialPrompt,omitempty"`
}

// OpencodeOut is the rendered frontmatter for an opencode agent file.
type OpencodeOut struct {
	Description string                 `yaml:"description"`
	Mode        string                 `yaml:"mode,omitempty"`
	Model       string                 `yaml:"model,omitempty"`
	Tools       map[string]bool        `yaml:"tools,omitempty"`
	Color       string                 `yaml:"color,omitempty"`
	Temperature interface{}            `yaml:"temperature,omitempty"`
	TopP        interface{}            `yaml:"top_p,omitempty"`
	Steps       interface{}            `yaml:"steps,omitempty"`
	Permission  map[string]interface{} `yaml:"permission,omitempty"`
	Disable     interface{}            `yaml:"disable,omitempty"`
	Hidden      interface{}            `yaml:"hidden,omitempty"`
}
