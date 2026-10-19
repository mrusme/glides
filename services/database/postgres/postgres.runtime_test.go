package postgres

import (
	"os"
	"path/filepath"
	"testing"

	"xn--gckvb8fzb.com/glides/runtime"
	"xn--gckvb8fzb.com/glides/services/database"
)

func testRuntime(t *testing.T, body string) *runtime.Runtime {
	t.Helper()

	path := filepath.Join(t.TempDir(), "app.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	rt, err := runtime.New(runtime.Opts{Cfgstr: path})
	if err != nil {
		t.Fatal(err)
	}

	return rt
}

func TestFromIsNilUntilRegistered(t *testing.T) {
	rt := testRuntime(t, "[Logging]\nLevel = \"error\"\n[Database]\nConnection = \"postgres://nobody@127.0.0.1:1/missing\"\n")

	if database.From(rt) != nil {
		t.Fatal("From returned a database that was never registered")
	}

	db, err := Register(rt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if database.From(rt) != db {
		t.Error("From did not return the registered database")
	}
}

func TestRegisterRejectsABrokenConnectionString(t *testing.T) {
	rt := testRuntime(t, "[Logging]\nLevel = \"error\"\n[Database]\nConnection = \"not a connection string\"\n")

	if _, err := Register(rt, Options{}); err == nil {
		t.Error("Register accepted a connection string that doesn't parse")
	}
	if database.From(rt) != nil {
		t.Error("a failed Register left a database behind")
	}
}
