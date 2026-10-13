package xmppclient

import (
	"context"
	"encoding/xml"
	"log/slog"
	"strings"

	goxmpp "github.com/xmppo/go-xmpp"
)

func (c *Client) receive() {
	defer close(c.done)
	defer close(c.messages)

	for {
		stanza, err := c.session.Recv()
		if err != nil {
			level := slog.LevelInfo
			select {
			case <-c.closing:
				level = slog.LevelDebug
			default:
			}
			c.log.Log(context.Background(), level, "XMPPClient.Recv", "status", "stream ended", "error", err)
			c.session.Close()
			return
		}

		c.handle(stanza)
	}
}

func (c *Client) handle(stanza any) {
	switch v := stanza.(type) {
	case goxmpp.IQ:
		if v.Type != "result" || v.ID == "" {
			return
		}
		c.pingMu.Lock()
		pending := c.pending
		c.pingMu.Unlock()
		if v.ID == pending {
			select {
			case c.pong <- v.ID:
			default:
			}
		}
	case goxmpp.Chat:
		switch v.Type {
		case "error":
			c.log.Warn("XMPPClient.Recv", "status", "message rejected",
				"remote", v.Remote, "condition", condition(v.OtherElem))
		case "chat", "normal", "":
			if v.Text == "" {
				return
			}
			if c.onMessage.Load() == nil {
				c.log.Debug("XMPPClient.Recv", "status", "message dropped",
					"remote", v.Remote, "bytes", len(v.Text))
				return
			}
			select {
			case c.messages <- Message{From: v.Remote, Body: v.Text}:
			default:
				c.log.Warn("XMPPClient.Recv", "status", "message dropped, the callback is behind",
					"remote", v.Remote, "queued", len(c.messages))
			}
		}
	case goxmpp.Presence:
		c.log.Debug("XMPPClient.Recv", "status", "presence", "remote", v.From, "type", v.Type)
	}
}

func (c *Client) deliver() {
	defer close(c.delivered)

	for m := range c.messages {
		if fn := c.onMessage.Load(); fn != nil {
			(*fn)(m)
			continue
		}
		c.log.Debug("XMPPClient.deliver", "status", "message dropped",
			"remote", m.From, "bytes", len(m.Body))
	}
}

func condition(elems []goxmpp.XMLElement) string {
	for _, el := range elems {
		if el.XMLName.Local != "error" {
			continue
		}

		dec := xml.NewDecoder(strings.NewReader(el.InnerXML))
		for {
			tok, err := dec.Token()
			if err != nil {
				break
			}
			if se, ok := tok.(xml.StartElement); ok {
				return se.Name.Local
			}
		}

		for _, attr := range el.Attr {
			if attr.Name.Local == "type" {
				return attr.Value
			}
		}
	}

	return "unknown"
}
