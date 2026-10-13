package xmpp

import (
	"context"

	"xn--gckvb8fzb.com/glides/models/asyncjob"
)

func (t *XMPP) PrepareMessage(
	jobType asyncjob.JobType,
	jobSubType asyncjob.JobSubType,
	envFrom string,
	rcptUsername string,
	rcptAddress string,
	lang string,
	subject string,
	data any,
) (message *Msg, err error) {
	message = NewMsg()

	message.To(rcptAddress)

	item, err := t.tmplCache.TemplatesFor(
		jobType,
		jobSubType,
		lang,
	)
	if err != nil {
		return nil, err
	}

	if err = message.SetBodyTextTemplate(
		item.TextTmpl,
		data,
	); err != nil {
		return nil, err
	}

	return message, nil
}

func (t *XMPP) SendMessages(
	ctx context.Context,
	messages []*Msg,
) (err error) {
	t.rt.Debug("execute", "send", "count", len(messages))

	t.mu.Lock()
	defer t.mu.Unlock()

	for _, message := range messages {
		if err = ctx.Err(); err != nil {
			return err
		}

		if err = t.ensureConnected(); err != nil {
			return err
		}

		if err = t.client.Send(message.to, message.ToString()); err != nil {
			t.rt.Error("failed to send", "xmpp",
				"error", err)
			return err
		}

		t.rt.Debug("successfully sent message", "xmpp",
			"destinationUsername", message.to)
	}

	return nil
}

func (t *XMPP) ensureConnected() error {
	if t.client != nil {
		select {
		case <-t.client.Done():
			t.rt.Info("connection ended, reconnecting ...", "xmpp",
				"host", t.opts.Host)
		default:
			err := t.client.Ping()
			if err == nil {
				return nil
			}
			t.rt.Error("ping failed, reconnecting ...", "xmpp",
				"error", err)
		}

		t.disconnect()
	}

	return t.connect()
}

func (t *XMPP) connect() (err error) {
	t.rt.Debug("connect to server ...", "xmpp",
		"host", t.opts.Host)

	opts := t.opts
	opts.OnMessage = t.onMessage

	if t.client, err = t.dial(opts); err != nil {
		t.client = nil
		return err
	}

	return nil
}

func (t *XMPP) disconnect() {
	if t.client == nil {
		return
	}

	t.rt.Debug("close existing client", "xmpp")

	t.client.Close()
	t.client = nil
}
