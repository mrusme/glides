package storage

import (
	"fmt"
	"io"
	"strings"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/services/config"
	"xn--gckvb8fzb.com/glides/services/storage/local"
	"xn--gckvb8fzb.com/glides/services/storage/s3"
)

type IStorage interface {
	Startup() error
	Shutdown() error
	StoreFile(src io.ReadSeeker, dest string) error
	StoreFileName(src string, dest string) error
	GetFileDownloadURL(dest string) (string, bool, error)
	GetFile(dest string) (io.ReadCloser, error)
	DeleteFile(dest string) error
}

type Storage struct {
	cfg       config.Storages
	providers map[string]IStorage
}

func New(cfg config.Storages) (str *Storage, err error) {
	str = new(Storage)

	str.cfg = cfg
	str.providers = make(map[string]IStorage)

	for _, cfg := range str.cfg {
		if _, exists := str.providers[cfg.ID]; exists {
			return nil, fmt.Errorf("%w: %s", errs.ErrStorageIDDuplicate, cfg.ID)
		}

		switch strings.ToLower(cfg.Type) {
		case "s3":
			if str.providers[cfg.ID], err = s3.New(cfg); err != nil {
				return nil, err
			}
		case "local":
			if str.providers[cfg.ID], err = local.New(cfg); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%w: %s", errs.ErrStorageTypeInvalid, cfg.Type)
		}
	}

	return str, nil
}

func (str *Storage) Startup() (err error) {
	for _, provider := range str.providers {
		if err = provider.Startup(); err != nil {
			return err
		}
	}
	return nil
}

func (str *Storage) Shutdown() (err error) {
	for _, provider := range str.providers {
		if err = provider.Shutdown(); err != nil {
			return err
		}
	}
	return nil
}

func (str *Storage) provider(id string) (IStorage, error) {
	provider, ok := str.providers[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errs.ErrStorageIDNotFound, id)
	}

	return provider, nil
}

func (str *Storage) StoreFile(
	providerID string,
	src io.ReadSeeker,
	dest string,
) (err error) {
	provider, err := str.provider(providerID)
	if err != nil {
		return err
	}

	return provider.StoreFile(src, dest)
}

func (str *Storage) StoreFileName(
	providerID string,
	src string,
	dest string,
) (err error) {
	provider, err := str.provider(providerID)
	if err != nil {
		return err
	}

	return provider.StoreFileName(src, dest)
}

func (str *Storage) GetFileDownloadURL(
	providerID string,
	dest string,
) (dlurl string, abs bool, err error) {
	provider, err := str.provider(providerID)
	if err != nil {
		return dlurl, false, err
	}

	return provider.GetFileDownloadURL(dest)
}

func (str *Storage) GetFile(
	providerID string,
	dest string,
) (reader io.ReadCloser, err error) {
	provider, err := str.provider(providerID)
	if err != nil {
		return nil, err
	}

	return provider.GetFile(dest)
}

func (str *Storage) DeleteFile(
	providerID string,
	dest string,
) (err error) {
	provider, err := str.provider(providerID)
	if err != nil {
		return err
	}

	return provider.DeleteFile(dest)
}
