package common

type ServerConfig struct {
	ID        string            `json:"id"`
	Workspace string            `json:"workspace"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Transport string            `json:"transport"`
	Cmd       string            `json:"cmd"` // for stdio
	Env       []string          `json:"env"`
	Url       string            `json:"url"` // for sse
	Headers   map[string]string `json:"headers"`
	Params    map[string]string `json:"params"` // for paramaterized cmd or url
}
