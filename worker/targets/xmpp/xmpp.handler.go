package xmpp

import (
	"xn--gckvb8fzb.com/glides/models/asyncjob"
	"xn--gckvb8fzb.com/glides/worker/targets/handler"
)

func HandlerFor[P any](
	t *XMPP,
	message func(payload P) handler.Message,
) handler.Handler {
	return func(job asyncjob.AsyncJob) (err error) {
		t.rt.Info("execute target", "xmpp",
			"type", job.Type, "sub_type", job.SubType)

		payloads, err := asyncjob.Payloads[P](job)
		if err != nil {
			return err
		}

		t.rt.Debug("payloads", payloads)

		var messages []*Msg
		for _, payload := range payloads {
			var msg *Msg
			m := message(payload)

			if msg, err = t.PrepareMessage(
				job.Type,
				job.SubType,
				"",
				m.Username,
				m.Address,
				m.Lang,
				m.Subject,
				payload,
			); err != nil {
				return err
			}

			messages = append(messages, msg)
		}

		return t.SendMessages(messages)
	}
}
