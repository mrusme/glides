package xmppclient

import (
	"crypto/rand"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"

	"xn--gckvb8fzb.com/glides/errs"
)

func escape(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))

	return b.String()
}

func (c *Client) Ping() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed() {
		return errs.ErrXMPPClosed
	}

	id := rand.Text()
	c.pingMu.Lock()
	c.pending = id
	c.pingMu.Unlock()

	select {
	case <-c.pong:
	default:
	}

	if _, err := c.session.SendOrg(fmt.Sprintf(
		"<iq from='%s' to='%s' id='%s' type='get'><ping xmlns='urn:xmpp:ping'/></iq>",
		escape(c.jid), escape(c.domain), id,
	)); err != nil {
		return err
	}

	timer := time.NewTimer(c.opts.PingTimeout)
	defer timer.Stop()

	select {
	case <-c.pong:
		return nil
	case <-c.done:
		return errs.ErrXMPPClosed
	case <-timer.C:
		return errs.ErrXMPPPingTimeout
	}
}

func (c *Client) keep() {
	ticker := time.NewTicker(c.opts.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-c.closing:
			return
		case <-ticker.C:
		}

		if err := c.Ping(); err != nil {
			if errors.Is(err, errs.ErrXMPPClosed) {
				return
			}
			c.log.Warn("XMPPClient.Ping", "status", "no reply, closing", "error", err)
			c.Close()
			return
		}
	}
}
