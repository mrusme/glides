package config

import "log/slog"

type Storage struct {
	ID    string       `koanf:"ID"`
	Type  string       `koanf:"Type"`
	S3    StorageS3    `koanf:"S3,omitempty"`
	Local StorageLocal `koanf:"Local,omitempty"`
}

type StorageS3 struct {
	Endpoint          string
	Region            string
	AccessKey         string
	SecretKey         string
	PublicURL         string
	PublicDownload    bool
	PresignedDownload bool
	PresignedExpiry   int
}

type StorageLocal struct {
	Path      string
	PublicURI string
}

type Storages []Storage

func (cfg *Config) Storages() (storages Storages, err error) {
	err = cfg.k.Load().Unmarshal("Storage", &storages)
	return storages, err
}

func (s Storage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("id", s.ID),
		slog.String("type", s.Type),
		slog.String("s3_endpoint", s.S3.Endpoint),
		slog.String("s3_region", s.S3.Region),
		slog.String("s3_public_url", s.S3.PublicURL),
		slog.String("local_path", s.Local.Path),
		slog.String("local_public_uri", s.Local.PublicURI),
	)
}
