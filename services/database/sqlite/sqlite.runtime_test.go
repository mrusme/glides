package sqlite

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"xn--gckvb8fzb.com/glides/errs"
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

func TestRegisterReadsThePath(t *testing.T) {
	file := filepath.Join(t.TempDir(), "app.db")
	rt := testRuntime(t, "[Logging]\nLevel = \"error\"\n[Database]\nPath = \""+file+"\"\n")

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
	if err = db.Startup(); err != nil {
		t.Fatal(err)
	}
	defer db.Shutdown()
	if _, err = os.Stat(file); err != nil {
		t.Errorf("Startup did not create the file of Database.Path: %v", err)
	}
}

func TestRegisterRejectsAMissingPath(t *testing.T) {
	rt := testRuntime(t, "[Logging]\nLevel = \"error\"\n")

	if _, err := Register(rt, Options{}); !errors.Is(err, errs.ErrDatabaseOptionsInvalid) {
		t.Errorf("Register without Database.Path: %v", err)
	}
	if database.From(rt) != nil {
		t.Error("a failed Register left a database behind")
	}
}
