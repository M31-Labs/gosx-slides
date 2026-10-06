package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"unicode/utf8"

	slides "m31labs.dev/gosx-slides"
)

// Stdio MCP is newline-delimited JSON-RPC. stdout belongs only to protocol
// messages; existing compiler warnings use stderr. No tools run shell commands.
func mcpCommand(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: slides mcp [deck-dir]")
	}
	project, err := slides.NewAuthorProject(deckDir(args))
	if err != nil {
		return err
	}
	return serveMCP(project, os.Stdin, os.Stdout)
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func mcpSchema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
func mcpTools() []map[string]any {
	str := map[string]string{"type": "string"}
	bounded := map[string]any{"type": "string", "maxLength": 1048576}
	tool := func(name, description string, schema map[string]any, writes bool) map[string]any {
		return map[string]any{"name": name, "description": description, "inputSchema": schema, "annotations": map[string]any{"readOnlyHint": !writes, "destructiveHint": writes, "openWorldHint": false}}
	}
	return []map[string]any{
		tool("slides_project_list", "List bounded editable author files and project revision; private state and symlinks are excluded.", mcpSchema(map[string]any{}), false),
		tool("slides_project_read", "Read one listed original author file with its SHA-256 and project revision.", mcpSchema(map[string]any{"file": str}, "file"), false),
		tool("slides_project_diagnose", "Diagnose the saved project or one unsaved file replacement; findings retain original filenames/ranges.", mcpSchema(map[string]any{"file": str, "source": bounded}), false),
		tool("slides_project_write", "Validate and save one original file, requiring both read revisions. Retains prior file; conflicts preserve the draft.", mcpSchema(map[string]any{"file": str, "source": bounded, "revision": str, "contextRevision": str}, "file", "source", "revision", "contextRevision"), true),
		tool("slides_project_rename", "Return mdpp scoped rename edits as an unsaved deck.md draft. Includes/cross-file rename are refused; write separately to publish.", mcpSchema(map[string]any{"file": str, "source": bounded, "offset": map[string]any{"type": "integer", "minimum": 0}, "name": str}, "file", "source", "offset", "name"), false),
		tool("slides_story_assert", "Run static compiled story assertions. Actual browser pose assertions use slides story assert --browser.", mcpSchema(map[string]any{}), false),
		tool("slides_resolve_address", "Resolve a stable slide/cue into a local URL anchor without broadcasting navigation.", mcpSchema(map[string]any{"slide": str, "cue": str}, "slide"), false),
		tool("slides_export_snapshot", "Export a private-notes-free snapshot or handout to a fresh private .slides-export-* folder. No external processes.", mcpSchema(map[string]any{"format": map[string]any{"type": "string", "enum": []string{"single", "handout"}}}, "format"), true),
	}
}

func serveMCP(project *slides.AuthorProject, input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 6*(1<<20)+16384)
	encoder := json.NewEncoder(output)
	initialized, ready := false, false
	respond := func(id json.RawMessage, result any, code int, message string) error {
		if id == nil {
			id = json.RawMessage("null")
		}
		response := map[string]any{"jsonrpc": "2.0", "id": id}
		if code != 0 {
			response["error"] = map[string]any{"code": code, "message": message}
		} else {
			response["result"] = result
		}
		return encoder.Encode(response)
	}
	for scanner.Scan() {
		var request mcpRequest
		if !utf8.Valid(scanner.Bytes()) || !json.Valid(scanner.Bytes()) {
			if err := respond(nil, nil, -32700, "invalid JSON-RPC message"); err != nil {
				return err
			}
			continue
		}
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			if err := respond(nil, nil, -32600, "invalid JSON-RPC request"); err != nil {
				return err
			}
			continue
		}
		validID := request.ID == nil
		if request.ID != nil {
			var value any
			decoder := json.NewDecoder(bytes.NewReader(request.ID))
			decoder.UseNumber()
			if decoder.Decode(&value) == nil {
				switch v := value.(type) {
				case string:
					validID = true
				case json.Number:
					_, err := strconv.ParseInt(v.String(), 10, 64)
					validID = err == nil
				}
			}
		}
		if request.JSONRPC != "2.0" || request.Method == "" || !validID {
			if err := respond(nil, nil, -32600, "invalid JSON-RPC request"); err != nil {
				return err
			}
			continue
		}
		if request.ID == nil {
			if request.Method == "notifications/initialized" && initialized {
				ready = true
			}
			continue // Notifications never receive responses or execute tools.
		}
		if request.Method == "initialize" {
			if initialized {
				if err := respond(request.ID, nil, -32600, "already initialized"); err != nil {
					return err
				}
				continue
			}
			var params struct {
				ProtocolVersion string         `json:"protocolVersion"`
				Capabilities    map[string]any `json:"capabilities"`
				ClientInfo      *struct {
					Name    string `json:"name"`
					Version string `json:"version"`
				} `json:"clientInfo"`
			}
			if err := json.Unmarshal(request.Params, &params); err != nil || params.ProtocolVersion == "" || params.ClientInfo == nil || params.ClientInfo.Name == "" || params.ClientInfo.Version == "" || params.Capabilities == nil {
				if err := respond(request.ID, nil, -32602, "initialize needs protocolVersion, capabilities and clientInfo"); err != nil {
					return err
				}
				continue
			}
			version := params.ProtocolVersion
			switch version {
			case "2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25":
			default:
				version = "2025-11-25"
			}
			initialized = true
			if err := respond(request.ID, map[string]any{"protocolVersion": version, "capabilities": map[string]any{"tools": map[string]any{"listChanged": false}}, "serverInfo": map[string]string{"name": "gosx-slides", "version": versionString()}, "instructions": "Read revisions before writes. Only listed original author files may be edited; source and speaker notes are private. Tools never broadcast presentation navigation."}, 0, ""); err != nil {
				return err
			}
			continue
		}
		if request.Method == "ping" {
			if err := respond(request.ID, map[string]any{}, 0, ""); err != nil {
				return err
			}
			continue
		}
		if !ready {
			if err := respond(request.ID, nil, -32600, "initialize and notifications/initialized are required"); err != nil {
				return err
			}
			continue
		}
		switch request.Method {
		case "tools/list":
			if err := respond(request.ID, map[string]any{"tools": mcpTools()}, 0, ""); err != nil {
				return err
			}
		case "tools/call":
			var params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
				Meta      json.RawMessage `json:"_meta,omitempty"`
			}
			if err := json.Unmarshal(request.Params, &params); err != nil || params.Name == "" {
				if err := respond(request.ID, nil, -32602, "tools/call needs a name and object arguments"); err != nil {
					return err
				}
				continue
			}
			known := false
			for _, tool := range mcpTools() {
				if tool["name"] == params.Name {
					known = true
					break
				}
			}
			if !known {
				if err := respond(request.ID, nil, -32602, "unknown tool"); err != nil {
					return err
				}
				continue
			}
			result, toolErr := callMCPTool(project, params.Name, params.Arguments)
			payload := map[string]any{}
			if toolErr != nil {
				payload["isError"] = true
				payload["content"] = []any{map[string]string{"type": "text", "text": toolErr.Error()}}
			} else {
				encoded, err := json.Marshal(result)
				if err != nil {
					return err
				}
				payload["content"] = []any{map[string]string{"type": "text", "text": string(encoded)}}
				payload["structuredContent"] = result
			}
			if err := respond(request.ID, payload, 0, ""); err != nil {
				return err
			}
		default:
			if err := respond(request.ID, nil, -32601, "method not found"); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func versionString() string { return version }

func callMCPTool(project *slides.AuthorProject, name string, args json.RawMessage) (any, error) {
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	if bytes.Equal(bytes.TrimSpace(args), []byte("null")) {
		return nil, fmt.Errorf("arguments must be an object")
	}
	decode := func(value any) error {
		decoder := json.NewDecoder(bytes.NewReader(args))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(value); err != nil {
			return err
		}
		if decoder.Decode(new(any)) != io.EOF {
			return fmt.Errorf("expected one argument object")
		}
		return nil
	}
	switch name {
	case "slides_project_list", "slides_story_assert":
		if err := decode(&struct{}{}); err != nil {
			return nil, err
		}
		if name == "slides_story_assert" {
			return project.AssertStory()
		}
		return project.List()
	case "slides_project_read":
		var input struct {
			File string `json:"file"`
		}
		if err := decode(&input); err != nil {
			return nil, err
		}
		return project.Read(input.File)
	case "slides_project_diagnose":
		var input struct {
			File   string  `json:"file"`
			Source *string `json:"source"`
		}
		if err := decode(&input); err != nil {
			return nil, err
		}
		if (input.File == "") != (input.Source == nil) {
			return nil, fmt.Errorf("file and source must be provided together")
		}
		source := ""
		if input.Source != nil {
			source = *input.Source
		}
		return project.Diagnose(input.File, source)
	case "slides_project_write":
		var edit slides.ProjectEdit
		if err := decode(&edit); err != nil {
			return nil, err
		}
		return project.Write(edit)
	case "slides_project_rename":
		var input struct {
			File   string `json:"file"`
			Source string `json:"source"`
			Offset int    `json:"offset"`
			Name   string `json:"name"`
		}
		if err := decode(&input); err != nil {
			return nil, err
		}
		return project.RenameDraft(input.File, input.Source, input.Offset, input.Name)
	case "slides_resolve_address":
		var input struct {
			Slide string `json:"slide"`
			Cue   string `json:"cue"`
		}
		if err := decode(&input); err != nil {
			return nil, err
		}
		return project.ResolveAddress(input.Slide, input.Cue)
	case "slides_export_snapshot":
		var input struct {
			Format string `json:"format"`
		}
		if err := decode(&input); err != nil {
			return nil, err
		}
		return project.ExportSnapshot(input.Format)
	}
	return nil, fmt.Errorf("unknown tool")
}
