package xmppclient

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

const envXMPP = "GLIDES_TEST_XMPP"

func liveOptions(t *testing.T) (opts Options, domain string) {
	t.Helper()

	raw := os.Getenv(envXMPP)
	if raw == "" {
		t.Skipf("%s is not set", envXMPP)
	}

	u, err := url.Parse(raw)
	if err != nil || u.User == nil || u.Host == "" {
		t.Fatalf("%s must be xmpp://user:password@host:port with an optional ?insecure=1: %v", envXMPP, err)
	}
	password, _ := u.User.Password()

	return Options{
		Host:               u.Host,
		User:               u.User.Username() + "@" + u.Hostname(),
		Password:           password,
		InsecureSkipVerify: u.Query().Get("insecure") == "1",
	}, u.Hostname()
}

func expectMessage(t *testing.T, from <-chan Message, body string, sender string) {
	t.Helper()

	select {
	case m := <-from:
		if m.Body != body || m.From != sender {
			t.Errorf("received %+v, want %q from %s", m, body, sender)
		}
	case <-time.After(5 * time.Second):
		t.Errorf("%q from %s never arrived", body, sender)
	}
}

func TestXMPPLiveSendsReceivesAndCloses(t *testing.T) {
	opts, domain := liveOptions(t)
	log, buf := testLog()

	received := make(chan Message, 4)
	opts.OnMessage = func(m Message) { received <- m }
	c, err := Dial(log, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if err = c.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	peerGot := make(chan Message, 4)
	peerOpts := opts
	peerOpts.OnMessage = func(m Message) { peerGot <- m }
	peer, err := Dial(nil, peerOpts)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	if err = c.Send(peer.JID(), "hello"); err != nil {
		t.Fatal(err)
	}
	expectMessage(t, peerGot, "hello", c.JID())

	if err = peer.Send(c.JID(), "hi back"); err != nil {
		t.Fatal(err)
	}
	expectMessage(t, received, "hi back", peer.JID())

	if err = c.Send("nobody@"+domain, "to nobody"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, buf, "message rejected")
	if !strings.Contains(buf.String(), "condition=service-unavailable") {
		t.Errorf("the rejection was logged as:\n%s", buf.String())
	}

	if err = peer.Close(); err != nil {
		t.Errorf("Close of the peer: %v", err)
	}
	started := time.Now()
	if err = c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	select {
	case <-c.Done():
	default:
		t.Error("Done isn't closed after Close")
	}
	if time.Since(started) > 2*time.Second {
		t.Errorf("Close took %v", time.Since(started))
	}
}

func TestXMPPLiveStaysConnectedWhileIdle(t *testing.T) {
	opts, _ := liveOptions(t)
	opts.PingInterval = 10 * time.Second

	c, err := Dial(nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	select {
	case <-c.Done():
		t.Fatal("the connection ended while idle")
	case <-time.After(30 * time.Second):
	}
	if err = c.Ping(); err != nil {
		t.Errorf("Ping after an idle half minute: %v", err)
	}

	got := make(chan Message, 1)
	peerOpts := opts
	peerOpts.OnMessage = func(m Message) { got <- m }
	peer, err := Dial(nil, peerOpts)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	if err = c.Send(peer.JID(), "after an idle half minute"); err != nil {
		t.Fatal(err)
	}
	expectMessage(t, got, "after an idle half minute", c.JID())
}
