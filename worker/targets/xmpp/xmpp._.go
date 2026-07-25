package xmpp

import (
	"crypto/tls"
	"strings"
	"sync"

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
	t = new(XMPP)

	t.rt = rt
	t.def = def
	t.tmplCache = tmpl.NewCache(rt.GetEmbed("templates"), tmpl.XMPPSpec)

	return t, nil
}

func (t *XMPP) Load() error {
	t.rt.Info("load target", "xmpp")
	t.rt.Debug("config", t.def)

	xmppServer := t.def.XMPP.Server
	xmppUsername := t.def.XMPP.Username
	xmppPassword := t.def.XMPP.Password

	t.jabberOpts = goxmpp.Options{
		Host:     xmppServer,
		User:     xmppUsername,
		Password: xmppPassword,
		NoTLS:    true,
		StartTLS: true,
		TLSConfig: &tls.Config{
			ServerName:         strings.Split(xmppServer, ":")[0],
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
	t.rt.Info("run target", "xmpp")

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
	t.rt.Info("shutdown target", "xmpp")

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.jabber == nil {
		return nil
	}

	return t.disconnect()
}
