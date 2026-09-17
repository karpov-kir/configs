package mcp

import (
	"encoding/json"
	"fmt"
	"os"
)

// The one key a declaration file carries.
const ServersKey = "mcpServers"

// Server is one entry of a declaration: its name, and its configuration exactly as the file wrote it.
// The configuration stays raw because it is what a client is handed — re-encoding it through a Go
// type would drop any field this tool does not model, and the whole point of the refusals in
// mcp-sync is that an unmodelled field is refused rather than silently lost.
type Server struct {
	Name   string
	Config json.RawMessage
}

// Document is one declaration file, read and substituted.
type Document struct {
	// Source is the file it came from, so a refusal can name which of the two files is at fault.
	Source  string
	Servers []Server
	// Keys is every top-level key the file carried. Codex refuses a file carrying anything but
	// mcpServers, so the refusal needs to see them.
	Keys []string
}

// ReadDocument reads one declaration file, strips its comments, substitutes the configs directory and
// returns its servers in document order.
//
// The caller has already established that the directory is substitutable — ConfigsDirIsSubstitutable
// is not re-asked here, because a caller that skipped it wants a refusal it can word for itself
// rather than a parse error about a JSON string that closed early.
func ReadDocument(file, configsDir string) (*Document, error) {
	text, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return ParseDocument(file, SubstituteConfigsDir(StripComments(string(text)), configsDir))
}

// ParseDocument reads one already-substituted declaration.
func ParseDocument(source, text string) (*Document, error) {
	top, err := ParseObject([]byte(text))
	if err != nil {
		return nil, fmt.Errorf("%s must contain an %s object", source, ServersKey)
	}
	raw, held := top.Get(ServersKey)
	if !held {
		return nil, fmt.Errorf("%s must contain an %s object", source, ServersKey)
	}
	servers, err := ParseObject(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must contain an %s object", source, ServersKey)
	}
	document := &Document{Source: source, Keys: top.Keys()}
	for _, name := range servers.Keys() {
		config, _ := servers.Get(name)
		document.Servers = append(document.Servers, Server{Name: name, Config: config})
	}
	return document, nil
}

// Stdio is the transport a declaration names when a server is a child process, and the only one
// `ai/project-mcp.sh` maps into a project.
const Stdio = "stdio"

// Transport is the server's declared `type`, defaulting to stdio the way both clients do.
func (s Server) Transport() string {
	var declared struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(s.Config, &declared); err != nil || declared.Type == "" {
		return Stdio
	}
	return declared.Type
}

// Command is the declared `command`, empty where the entry declares none.
func (s Server) Command() string {
	var declared struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(s.Config, &declared); err != nil {
		return ""
	}
	return declared.Command
}
