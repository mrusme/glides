package database

import (
	"os"
	"path/filepath"
	"testing"

	"xn--gckvb8fzb.com/glides/runtime"
)

func TestFromIsNilUntilRegistered(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.toml")
	if err := os.WriteFile(path, []byte("[Logging]\nLevel = \"error\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rt, err := runtime.New(runtime.Opts{Cfgstr: path})
	if err != nil {
		t.Fatal(err)
	}

	if From(rt) != nil {
		t.Fatal("From returned a database that was never registered")
	}

	db := New(nil, new(fakeDriver))
	rt.Register(ServiceID, db)
	if From(rt) != db {
		t.Error("From did not return the registered database")
	}
}
