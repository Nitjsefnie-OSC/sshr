package app

import "testing"

func TestParseForward(t *testing.T) {
	good := []string{
		"db -L 5432:localhost:5432",
		"db2 -L 127.0.0.1:15432:db.internal:5432",
		"web -R 8080:localhost:3000",
		"v6 -L 2222:[::1]:22",
		"socks -D 1080",
		"socks2 -D 127.0.0.1:1080",
	}
	for _, s := range good {
		f, err := ParseForward(s)
		if err != nil {
			t.Errorf("ParseForward(%q) = %v", s, err)
			continue
		}
		if f.String() != s {
			t.Errorf("String() = %q, want %q", f.String(), s)
		}
	}
	bad := []string{
		"db",
		"db -L",
		"db -X 1:h:2",
		"db -L 5432:localhost",
		"db -L 70000:localhost:5432",
		"db -L 5432::5432",
		"db -D abc",
		"bad name -L 1:h:2",
		"-x -L 1:h:2",
	}
	for _, s := range bad {
		if _, err := ParseForward(s); err == nil {
			t.Errorf("ParseForward(%q) accepted", s)
		}
	}
}

func TestForwardsOnHost(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	fs, err := ParseForwards("db -L 5432:localhost:5432\n\nsocks -D 1080\n")
	if err != nil || len(fs) != 2 {
		t.Fatalf("ParseForwards = %v, %v", fs, err)
	}
	h, err := s.Add(Host{Host: "h", User: "u", Forwards: fs})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.Find(h.ID)
	if FormatForwards(got.Forwards) != "db -L 5432:localhost:5432\nsocks -D 1080" {
		t.Errorf("saved forwards = %q", FormatForwards(got.Forwards))
	}
	if _, err := s.Add(Host{Host: "h", User: "u", Forwards: append(fs, fs[0])}); err == nil {
		t.Error("duplicate forward name accepted")
	}
	pick, err := FindForwards(fs, []string{"socks"})
	if err != nil || len(pick) != 1 || pick[0].Name != "socks" {
		t.Errorf("FindForwards(socks) = %v, %v", pick, err)
	}
	if _, err := FindForwards(fs, []string{"nope"}); err == nil {
		t.Error("FindForwards accepted an unknown name")
	}
}
