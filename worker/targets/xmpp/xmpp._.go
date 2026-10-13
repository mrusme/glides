package xmpp

import (
	"fmt"
	"sync"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/helpers/xmppclient"
	"xn--gckvb8fzb.com/glides/runtime"
	"xn--gckvb8fzb.com/glides/services/config"
	"xn--gckvb8fzb.com/glides/worker/targets/handler"
	"xn--gckvb8fzb.com/glides/worker/targets/tmpl"
)

type client interface {
	Send(to string, body string) error
	Ping() error
	Close() error
	Done() <-chan struct{}
	SetOnMessage(fn func(xmppclient.Message))
}

type XMPP struct {
	handler.Handlers

	rt        *runtime.Runtime
	def       config.Target
	tmplCache *tmpl.Cache

	opts      xmppclient.Options
	dial      func(opts xmppclient.Options) (client, error)
	onMessage func(xmppclient.Message)

	mu     sync.Mutex
	client client
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
	t.dial = func(opts xmppclient.Options) (client, error) {
		c, err := xmppclient.Dial(rt.Logger(), opts)
		if err != nil {
			return nil, err
		}

		return c, nil
	}

	return t, nil
}

func (t *XMPP) Load() error {
	t.rt.Info("load target", "xmpp", "id", t.def.ID)
	t.rt.Debug("config", t.def)

	t.opts = xmppclient.Options{
		Host:               t.def.XMPP.Server,
		User:               t.def.XMPP.Username,
		Password:           t.def.XMPP.Password,
		InsecureSkipVerify: t.def.XMPP.InsecureSkipVerify,
		Debug:              t.rt.IsDevelopmentMode(),
	}

	return nil
}

func (t *XMPP) OnMessage(fn func(xmppclient.Message)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.onMessage = fn
	if t.client != nil {
		t.client.SetOnMessage(fn)
	}
}

func (t *XMPP) Run() error {
	t.rt.Info("run target", "xmpp", "id", t.def.ID)

	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.connect(); err != nil {
		t.rt.Error("failed to connect, will retry on first send", "xmpp",
			"host", t.opts.Host,
			"error", err)
	}

	return nil
}

func (t *XMPP) Shutdown() error {
	t.rt.Info("shutdown target", "xmpp", "id", t.def.ID)

	t.mu.Lock()
	defer t.mu.Unlock()

	t.disconnect()

	return nil
}
