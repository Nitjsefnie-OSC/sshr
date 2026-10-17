package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"sshr.dev/internal/app"
)

// exchange sends one request per line and returns the answers by id
func exchange(t *testing.T, store *app.Store, opts Options, lines ...string) map[string]map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := Serve(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out, store, opts); err != nil {
		t.Fatal(err)
	}
	got := map[string]map[string]any{}
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("bad answer %q: %v", l, err)
		}
		id, _ := json.Marshal(m["id"])
		got[string(id)] = m
	}
	return got
}

func toolNames(m map[string]any) []string {
	var names []string
	for _, t := range m["result"].(map[string]any)["tools"].([]any) {
		names = append(names, t.(map[string]any)["name"].(string))
	}
	return names
}

func TestServe(t *testing.T) {
	s := &app.Store{Dir: t.TempDir()}
	if _, err := s.Add(app.Host{Name: "web", Host: "10.0.0.1", User: "u", PasswordMode: app.PasswordAsk, Jump: "bastion"}); err != nil {
		t.Fatal(err)
	}
	got := exchange(t, s, Options{},
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_hosts","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"run_command","arguments":{"host":"web","command":"id"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"nope"}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"upload_file","arguments":{"host":"nope","path":"/etc/hostname"}}}`,
		`{"jsonrpc":"2.0","id":"x","method":"initialize","params":{"protocolVersion":"1999-01-01"}}`,
	)
	if len(got) != 7 {
		t.Fatalf("answers = %d, want 7 (no answer to the notification)", len(got))
	}
	if v := got["1"]["result"].(map[string]any)["protocolVersion"]; v != "2025-06-18" {
		t.Errorf("negotiated %v", v)
	}
	if v := got[`"x"`]["result"].(map[string]any)["protocolVersion"]; v != versions[0] {
		t.Errorf("unknown version answered with %v", v)
	}
	if names := strings.Join(toolNames(got["2"]), ","); names != "list_hosts,upload_file" {
		t.Errorf("tools = %s", names)
	}
	text := got["3"]["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, `"name": "web"`) || !strings.Contains(text, "bastion") || !strings.Contains(text, "asked every time") {
		t.Errorf("list_hosts = %s", text)
	}
	if got["4"]["error"] == nil {
		t.Error("run_command answered while commands are off")
	}
	if got["5"]["error"].(map[string]any)["code"].(float64) != -32601 {
		t.Errorf("unknown method = %v", got["5"])
	}
	if r := got["6"]["result"].(map[string]any); r["isError"] != true {
		t.Errorf("upload to an unknown host = %v", r)
	}
}

func TestRunCommandOffered(t *testing.T) {
	s := &app.Store{Dir: t.TempDir()}
	s.Add(app.Host{Name: "web", Host: "10.0.0.1", User: "u", PasswordMode: app.PasswordAsk})
	got := exchange(t, s, Options{AllowCommands: true},
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"run_command","arguments":{"host":"web","command":"id"}}}`,
	)
	if names := strings.Join(toolNames(got["1"]), ","); names != "list_hosts,upload_file,run_command" {
		t.Errorf("tools = %s", names)
	}
	// a host that asks for its password has nobody to ask here
	r := got["2"]["result"].(map[string]any)
	if r["isError"] != true {
		t.Errorf("run on an ask host = %v", r)
	}
}
