package config

import (
	"os"
	"path/filepath"
	"testing"
)

func testConfig(t *testing.T, body string) *Config {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := New("file://" + path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return cfg
}

func TestUnmarshalReadsThePathItIsGiven(t *testing.T) {
	cfg := testConfig(t, `
[AuthProvider]
Key = "wrong"

[DAV]
BindIP = "0.0.0.0"
Port = 8443

[SMTP]
Domain = "mail.example.org"
`)

	var dav struct {
		BindIP string `koanf:"BindIP"`
		Port   int    `koanf:"Port"`
	}
	if _, err := cfg.Unmarshal("DAV", &dav); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got, want := dav.BindIP, "0.0.0.0"; got != want {
		t.Errorf("BindIP = %q, want %q; Unmarshal must read the section it is "+
			"given, not a hardcoded one, or every consumer silently gets a "+
			"zero value and falls back to its defaults",
			got, want)
	}
	if got, want := dav.Port, 8443; got != want {
		t.Errorf("Port = %d, want %d", got, want)
	}

	var smtp struct {
		Domain string `koanf:"Domain"`
	}
	if _, err := cfg.Unmarshal("SMTP", &smtp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got, want := smtp.Domain, "mail.example.org"; got != want {
		t.Errorf("Domain = %q, want %q", got, want)
	}
}

func TestUnmarshalOfAMissingSectionLeavesTheTargetZeroed(t *testing.T) {
	cfg := testConfig(t, "[General]\nMode = \"development\"\n")

	var dav struct {
		BindIP string `koanf:"BindIP"`
	}
	if _, err := cfg.Unmarshal("DAV", &dav); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if dav.BindIP != "" {
		t.Errorf("BindIP = %q, want empty for an absent section", dav.BindIP)
	}
}

func TestStrings(t *testing.T) {
	cfg := testConfig(t, "[Users]\nPromoteAdmin = [\"a@example.org\", \"b@example.org\"]\n")

	got := cfg.Strings("Users.PromoteAdmin")
	want := []string{"a@example.org", "b@example.org"}

	if len(got) != len(want) {
		t.Fatalf("Strings() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Strings()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
