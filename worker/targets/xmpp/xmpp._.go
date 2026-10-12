package xmpp

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/runtime"
	"xn--gckvb8fzb.com/glides/services/config"
	"xn--gckvb8fzb.com/glides/worker/targets/handler"
	"xn--gckvb8fzb.com/glides/worker/targets/tmpl"

	goxmpp "github.com/xmppo/go-xmpp"
)

type XMPP struct {
	handler.Handlers

	rt        *runtime.Runtime
	def       config.Target
	tmplCache *tmpl.Cache

	jabberOpts goxmpp.Options

	mu     sync.Mutex
	jabber *goxmpp.Client
}

func New(
	rt *runtime.Runtime,
	def config.Target,
) (t *XMPP, err error) {
	templates := rt.GetEmbed("templates")
	if templates == nil {
		return nil, fmt.Errorf("%w: templates", errs.ErrEmbedNotFound)
	}

	t = new(XMPP)

	t.rt = rt
	t.def = def
	t.tmplCache = tmpl.NewCache(templates, tmpl.XMPPSpec)

	return t, nil
}

func serverName(server string) string {
	if host, _, err := net.SplitHostPort(server); err == nil {
		return host
	}

	return server
}

func (t *XMPP) Load() error {
	t.rt.Info("load target", "xmpp", "id", t.def.ID)
	t.rt.Debug("config", t.def)

	t.jabberOpts = goxmpp.Options{
		Host:     t.def.XMPP.Server,
		User:     t.def.XMPP.Username,
		Password: t.def.XMPP.Password,
		NoTLS:    true,
		StartTLS: true,
		TLSConfig: &tls.Config{
			ServerName:         serverName(t.def.XMPP.Server),
			InsecureSkipVerify: t.def.XMPP.InsecureSkipVerify,
		},
		Debug:               t.rt.IsDevelopmentMode(),
		Session:             true,
		Status:              "chat",
		StatusMessage:       "",
		PeriodicServerPings: true,
	}

	return nil
}

func (t *XMPP) Run() error {
	t.rt.Info("run target", "xmpp", "id", t.def.ID)

	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.reconnect(); err != nil {
		t.rt.Error("failed to connect, will retry on first send", "xmpp",
			"host", t.jabberOpts.Host,
			"error", err)
	}

	return nil
}

func (t *XMPP) Shutdown() error {
	t.rt.Info("shutdown target", "xmpp", "id", t.def.ID)

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.jabber == nil {
		return nil
	}

	return t.disconnect()
}
