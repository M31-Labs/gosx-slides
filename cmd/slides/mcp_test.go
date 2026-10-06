package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	slides "m31labs.dev/gosx-slides"
)

func TestMCPProtocolLifecycleErrorsAndNotifications(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "deck.md"), []byte("# Local\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := slides.NewAuthorProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		`{bad}`,
		`{"jsonrpc":"2.0","id":{},"method":"ping"}`,
		`{"jsonrpc":"2.0","id":"hello","method":"initialize","params":{"protocolVersion":"2099-01-01","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"slides_project_write","arguments":{"file":"deck.md","source":"# Forged"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":999}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"slides_project_read","arguments":{"file":"../private.md"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"run_shell","arguments":{"command":"echo leaked"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"slides_project_list","arguments":{"command":"echo leaked"}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"unknown"}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	if err := serveMCP(p, strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	var replies []map[string]any
	for decoder.More() {
		var reply map[string]any
		if err := decoder.Decode(&reply); err != nil {
			t.Fatal(err)
		}
		replies = append(replies, reply)
	}
	if len(replies) != 9 {
		t.Fatalf("notifications emitted replies: %d", len(replies))
	}
	if replies[0]["error"].(map[string]any)["code"] != float64(-32600) || replies[1]["error"].(map[string]any)["code"] != float64(-32700) || replies[2]["id"] != nil {
		t.Fatalf("protocol errors: %+v", replies[:3])
	}
	if replies[3]["id"] != "hello" || replies[3]["result"].(map[string]any)["protocolVersion"] != "2025-11-25" {
		t.Fatalf("negotiation: %+v", replies[3])
	}
	tools := replies[4]["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 8 {
		t.Fatalf("tool schema count: %d", len(tools))
	}
	for _, index := range []int{5, 7} {
		if replies[index]["result"].(map[string]any)["isError"] != true {
			t.Fatalf("tool failure not represented: %+v", replies[index])
		}
	}
	if replies[6]["error"].(map[string]any)["code"] != float64(-32602) || replies[8]["error"].(map[string]any)["code"] != float64(-32601) {
		t.Fatalf("unknown tool/method: %+v", replies)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "deck.md"))
	if string(raw) != "# Local\n" {
		t.Fatal("notification executed mutation")
	}
}

func TestMCPStrictRequestsAndUTF8(t *testing.T) {
	input := "[]\n" + `{"jsonrpc":"2.0","id":1.0000000000000000001,"method":"ping"}` + "\n" + `{"jsonrpc":"2.0","id":null,"method":"ping"}` + "\n" + `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":4,"version":"1"}}}` + "\n" + string([]byte{'"', 0xff, '"', '\n'})
	var output bytes.Buffer
	if err := serveMCP(nil, strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	for i, want := range []int{-32600, -32600, -32600, -32602, -32700} {
		var reply struct {
			Error struct {
				Code int `json:"code"`
			} `json:"error"`
		}
		if err := decoder.Decode(&reply); err != nil || reply.Error.Code != want {
			t.Fatalf("response %d: code %d, expected %d, decode %v", i, reply.Error.Code, want, err)
		}
	}
}

func TestMCPRealProjectReadWriteAndConflict(t *testing.T) {
	dir := t.TempDir()
	initial := "```yaml\nid: opening\ncues: initial, done\n```\n\n# Opening\n"
	if err := os.WriteFile(filepath.Join(dir, "deck.md"), []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := slides.NewAuthorProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	read, err := callMCPTool(p, "slides_project_read", json.RawMessage(`{"file":"deck.md"}`))
	if err != nil {
		t.Fatal(err)
	}
	doc := read.(slides.ProjectDocument)
	edit := slides.ProjectEdit{File: doc.Path, Source: initial + "\nUpdated\n", Revision: doc.Revision, ContextRevision: doc.ContextRevision}
	encoded, _ := json.Marshal(edit)
	if _, err := callMCPTool(p, "slides_project_write", encoded); err != nil {
		t.Fatal(err)
	}
	if _, err := callMCPTool(p, "slides_project_write", encoded); err == nil {
		t.Fatal("MCP stale write accepted")
	}
	result, err := callMCPTool(p, "slides_resolve_address", json.RawMessage(`{"slide":"opening","cue":"done"}`))
	if err != nil || result.(map[string]any)["anchor"] != "#opening/done" {
		t.Fatalf("address: %+v %v", result, err)
	}
	if _, err := callMCPTool(p, "slides_project_diagnose", json.RawMessage(`{"file":"deck.md"}`)); err == nil {
		t.Fatal("incomplete draft diagnosed")
	}
	if _, err := callMCPTool(p, "slides_project_read", json.RawMessage(`null`)); err == nil {
		t.Fatal("null arguments accepted")
	}
	if _, err := callMCPTool(p, "slides_export_snapshot", json.RawMessage(`{"format":"handout","out":"../secret"}`)); err == nil {
		t.Fatal("user-defined output accepted")
	}
}

func TestMCPMessageSizeBound(t *testing.T) {
	dir := t.TempDir()
	p, err := slides.NewAuthorProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := serveMCP(p, strings.NewReader(strings.Repeat("x", 6*(1<<20)+20000)), &output); err == nil {
		t.Fatal("unbounded message accepted")
	}
	if output.Len() != 0 {
		t.Fatal("oversized input leaked to output")
	}
}
