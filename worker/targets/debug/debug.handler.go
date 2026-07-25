package debug

import (
	"xn--gckvb8fzb.com/glides/models/asyncjob"
	"xn--gckvb8fzb.com/glides/worker/targets/handler"
)

func HandlerFor[P any](
	t *Debug,
	message func(payload P) handler.Message,
) handler.Handler {
	return func(job asyncjob.AsyncJob) (err error) {
		t.rt.Info("execute target", "debug",
			"type", job.Type, "sub_type", job.SubType)

		payloads, err := asyncjob.Payloads[P](job)
		if err != nil {
			return err
		}

		var messages []*Msg
		for _, payload := range payloads {
			var msg *Msg
			m := message(payload)

			if msg, err = t.PrepareMessage(
				job,
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
