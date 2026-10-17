package app

import (
	"strings"
	"testing"
)

func TestCopyArgs(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	h := &Host{ID: "a", Name: "web", Host: "10.0.0.5", User: "deploy", Port: 2222, Key: "/k/id"}
	got, err := s.CopyArgs(h, true, []string{"a.txt", "dir"}, "/srv")
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Join(got, " ")
	if !strings.Contains(line, "-P 2222") || strings.Contains(line, "-p ") || !strings.HasSuffix(line, "-r -- a.txt dir deploy@10.0.0.5:/srv") {
		t.Fatalf("send = %s", line)
	}
	h6 := &Host{ID: "b", Host: "::1", User: "u", Port: 22}
	got, _ = s.CopyArgs(h6, false, []string{"/var/log/x"}, ".")
	if line = strings.Join(got, " "); !strings.HasSuffix(line, "-- u@[::1]:/var/log/x .") {
		t.Fatalf("get = %s", line)
	}
}
