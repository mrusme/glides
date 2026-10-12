package storage

import (
	"errors"
	"testing"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/services/config"
)

func TestNewValidatesTheProviders(t *testing.T) {
	if _, err := New(config.Storages{
		{ID: "a", Type: "Local", Local: config.StorageLocal{Path: t.TempDir()}},
		{ID: "a", Type: "S3"},
	}); !errors.Is(err, errs.ErrStorageIDDuplicate) {
		t.Errorf("two providers with one id = %v", err)
	}

	if _, err := New(config.Storages{{ID: "a", Type: "ftp"}}); !errors.Is(err, errs.ErrStorageTypeInvalid) {
		t.Errorf("an unknown type = %v", err)
	}

	str, err := New(config.Storages{{ID: "a", Type: "Local", Local: config.StorageLocal{Path: t.TempDir()}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = str.GetFileDownloadURL("missing", "x"); !errors.Is(err, errs.ErrStorageIDNotFound) {
		t.Errorf("an unknown provider id = %v", err)
	}
}
