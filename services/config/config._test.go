package config

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xn--gckvb8fzb.com/glides/errs"
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
	if err := cfg.Unmarshal("DAV", &dav); err != nil {
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
	if err := cfg.Unmarshal("SMTP", &smtp); err != nil {
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
	if err := cfg.Unmarshal("DAV", &dav); err != nil {
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

func TestGeneralModeDefaultsToProduction(t *testing.T) {
	cfg := testConfig(t, "[Logging]\nLevel = \"info\"\n")

	if got := cfg.GeneralMode(); got != ModeProduction {
		t.Errorf("GeneralMode() = %q without a [General] section, want %q", got, ModeProduction)
	}

	cfg = testConfig(t, "[General]\nMode = \"Development\"\n")
	if got := cfg.GeneralMode(); got != ModeDevelopment {
		t.Errorf("GeneralMode() = %q, want %q in lower case", got, ModeDevelopment)
	}
}

func TestNewRejectsAnUnknownMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.toml")
	if err := os.WriteFile(path, []byte("[General]\nMode = \"staging\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := New(path); !errors.Is(err, errs.ErrConfigModeInvalid) {
		t.Errorf("New with an unknown mode = %v, want %v", err, errs.ErrConfigModeInvalid)
	}
}

func TestLoggingLevelIsEmptyWhenAbsent(t *testing.T) {
	cfg := testConfig(t, "[General]\nMode = \"production\"\n")

	if got := cfg.LoggingLevel(); got != "" {
		t.Errorf("LoggingLevel() = %q, want empty", got)
	}
}

func TestRedisReadsEveryKey(t *testing.T) {
	cfg := testConfig(t, `
[Redis]
Addresses = ["127.0.0.1:6379"]
Database = 2
MasterName = "mymaster"
Username = "u"
Password = "p"
Poolsize = 7
Reset = true
`)

	r, err := cfg.Redis()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Addrs) != 1 || r.Database != 2 || r.MasterName != "mymaster" ||
		r.Username != "u" || r.Password != "p" || r.Poolsize != 7 || !r.Reset {
		t.Errorf("Redis() = %+v", r)
	}
}

func TestSecretsAreNotLogged(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	target := Target{ID: "t", Type: TargetTypeEmail}
	target.Email.SMTPPassword = "smtp-secret"
	target.XMPP.Password = "xmpp-secret"
	redis := Redis{Addrs: []string{"127.0.0.1:6379"}, Password: "redis-secret"}
	storage := Storage{ID: "s", Type: "S3"}
	storage.S3.AccessKey = "access-secret"
	storage.S3.SecretKey = "key-secret"

	log.Info("config", "target", target, "redis", redis, "storage", storage)

	for _, secret := range []string{"smtp-secret", "xmpp-secret", "redis-secret", "access-secret", "key-secret"} {
		if strings.Contains(buf.String(), secret) {
			t.Errorf("the log line contains %q: %s", secret, buf.String())
		}
	}
	for _, public := range []string{`"id":"t"`, `"addresses":["127.0.0.1:6379"]`, `"id":"s"`} {
		if !strings.Contains(buf.String(), public) {
			t.Errorf("the log line lacks %s: %s", public, buf.String())
		}
	}
}
