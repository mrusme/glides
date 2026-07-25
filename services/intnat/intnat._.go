package intnat

import (
	"embed"

	"github.com/kaptinlin/go-i18n"

	"github.com/pelletier/go-toml/v2"
)

const LocalesGlob string = "locales/*.toml"

const DefaultLocale string = "en"

type Intnat struct {
	Bundle  *i18n.I18n
	Locales *embed.FS

	defaultLocale string
	locales       []string
}

func New() (in *Intnat, err error) {
	in = new(Intnat)

	in.defaultLocale = DefaultLocale
	in.locales = []string{DefaultLocale}

	return in, nil
}

func (in *Intnat) SetLocales(
	embedLocales *embed.FS,
	defaultLocale string,
	locales ...string,
) (err error) {
	in.Locales = embedLocales

	if defaultLocale != "" {
		in.defaultLocale = defaultLocale
	}

	if len(locales) > 0 {
		in.locales = locales
	}

	return nil
}

func (in *Intnat) Startup() (err error) {
	if in.Bundle, err = i18n.NewBundle(
		i18n.WithDefaultLocale(in.defaultLocale),
		i18n.WithLocales(in.locales...),
		i18n.WithUnmarshaler(toml.Unmarshal),
	); err != nil {
		return err
	}

	if in.Locales == nil {
		return nil
	}

	return in.Bundle.LoadFS(in.Locales, LocalesGlob)
}

func (in *Intnat) Shutdown() (err error) {
	return nil
}

func (in *Intnat) NewLocalizer(locale string) *i18n.Localizer {
	return in.Bundle.NewLocalizer(in.Bundle.MatchAvailableLocale(locale))
}
