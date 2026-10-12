package local

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/services/config"
)

func testLocal(t *testing.T) *Local {
	t.Helper()

	st, err := New(config.Storage{ID: "l", Type: "Local", Local: config.StorageLocal{
		Path:      filepath.Join(t.TempDir(), "files"),
		PublicURI: "/media",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err = st.Startup(); err != nil {
		t.Fatal(err)
	}

	return st
}

func TestStartupNeedsAPath(t *testing.T) {
	st, err := New(config.Storage{ID: "l", Type: "Local"})
	if err != nil {
		t.Fatal(err)
	}
	if err = st.Startup(); !errors.Is(err, errs.ErrFilePathInvalid) {
		t.Errorf("Startup without a path = %v", err)
	}
}

func TestResolvePathStaysInsideTheBase(t *testing.T) {
	st := testLocal(t)

	for _, dest := range []string{"../x", "a/../../x", "../../etc/passwd"} {
		if _, err := st.resolvePath(dest); !errors.Is(err, errs.ErrFilePathInvalid) {
			t.Errorf("resolvePath(%q) = %v, want %v", dest, err, errs.ErrFilePathInvalid)
		}
	}

	for _, dest := range []string{"a/b.txt", "/a/b.txt", "a/../b.txt"} {
		got, err := st.resolvePath(dest)
		if err != nil || !strings.HasPrefix(got, st.cfg.Local.Path+string(os.PathSeparator)) {
			t.Errorf("resolvePath(%q) = %q, %v", dest, got, err)
		}
	}
}

func TestStoreGetDelete(t *testing.T) {
	st := testLocal(t)

	if err := st.StoreFile(strings.NewReader("hello"), "2026/10/hello.txt"); err != nil {
		t.Fatal(err)
	}

	url, abs, err := st.GetFileDownloadURL("2026/10/hello.txt")
	if err != nil || abs || url != "/media/2026/10/hello.txt" {
		t.Errorf("GetFileDownloadURL = %q, %v, %v", url, abs, err)
	}

	reader, err := st.GetFile("2026/10/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || string(body) != "hello" {
		t.Errorf("GetFile = %q, %v", body, err)
	}

	if err = st.DeleteFile("2026/10/hello.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err = st.GetFile("2026/10/hello.txt"); err == nil {
		t.Error("GetFile found a deleted file")
	}

	for _, dest := range []string{"", "../escape.txt"} {
		if err = st.StoreFile(strings.NewReader("x"), dest); !errors.Is(err, errs.ErrFilePathInvalid) {
			t.Errorf("StoreFile(%q) = %v", dest, err)
		}
	}
}
