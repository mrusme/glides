package config

import "log/slog"

const (
	TargetTypeEmail string = "email"
	TargetTypeXMPP  string = "xmpp"
	TargetTypeDebug string = "debug"
)

type Target struct {
	ID    string      `koanf:"ID"`
	Type  string      `koanf:"Type"`
	Email TargetEmail `koanf:"Email,omitempty"`
	XMPP  TargetXMPP  `koanf:"XMPP,omitempty"`
	Debug TargetDebug `koanf:"Debug,omitempty"`
}

type TargetEmail struct {
	SMTPServer    string
	SMTPAuthType  string
	SMTPTLSPolicy int
	SMTPUsername  string
	SMTPPassword  string
	From          struct {
		Email string
		Name  string
	}
}

type TargetXMPP struct {
	Server             string
	InsecureSkipVerify bool
	Username           string
	Password           string
}

type TargetDebug struct {
	Emulates string
	Path     string
}

func (t Target) IsDebug() bool {
	return t.Type == TargetTypeDebug
}

func (t Target) Serves(channel string) bool {
	if t.IsDebug() {
		return t.Debug.Emulates == channel
	}

	return t.Type == channel
}

func (t Target) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("id", t.ID),
		slog.String("type", t.Type),
		slog.String("email_smtp_server", t.Email.SMTPServer),
		slog.String("email_from", t.Email.From.Email),
		slog.String("xmpp_server", t.XMPP.Server),
		slog.String("xmpp_username", t.XMPP.Username),
		slog.String("debug_emulates", t.Debug.Emulates),
		slog.String("debug_path", t.Debug.Path),
	)
}

type Targets []Target

func (cfg *Config) Targets() (targets Targets, err error) {
	err = cfg.k.Load().Unmarshal("Target", &targets)
	return targets, err
}
