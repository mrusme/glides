package xmpp

import (
	"context"
	"embed"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/helpers/xmppclient"
	"xn--gckvb8fzb.com/glides/runtime"
	"xn--gckvb8fzb.com/glides/services/config"
)

type fakeClient struct {
	mu        sync.Mutex
	sent      []string
	pings     int
	pingErr   error
	closed    bool
	onMessage func(xmppclient.Message)
	done      chan struct{}
}

func (f *fakeClient) SetOnMessage(fn func(xmppclient.Message)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.onMessage = fn
}

func (f *fakeClient) Send(to string, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, to+": "+body)
	return nil
}

func (f *fakeClient) Ping() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pings++
	return f.pingErr
}

func (f *fakeClient) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeClient) Done() <-chan struct{} {
	return f.done
}

type dials struct {
	clients []*fakeClient
	opts    []xmppclient.Options
	err     error
}

func testTarget(t *testing.T) (*XMPP, *dials) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "app.toml")
	if err := os.WriteFile(path, []byte("[Logging]\nLevel = \"error\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rt, err := runtime.New(runtime.Opts{Cfgstr: path})
	if err != nil {
		t.Fatal(err)
	}
	rt.AddEmbed("templates", &embed.FS{})

	target, err := New(rt, config.Target{
		ID:   "jabber",
		Type: config.TargetTypeXMPP,
		XMPP: config.TargetXMPP{
			Server:             "jabber.example.org:5222",
			Username:           "bot@example.org",
			Password:           "swordfish",
			InsecureSkipVerify: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = target.Load(); err != nil {
		t.Fatal(err)
	}

	d := new(dials)
	target.dial = func(opts xmppclient.Options) (client, error) {
		if d.err != nil {
			return nil, d.err
		}
		c := &fakeClient{done: make(chan struct{})}
		d.clients = append(d.clients, c)
		d.opts = append(d.opts, opts)
		return c, nil
	}

	return target, d
}

func batch(to string, body string) []*Msg {
	return []*Msg{{to: to, body: body}}
}

func TestLoadBuildsTheOptionsFromTheTarget(t *testing.T) {
	target, _ := testTarget(t)

	o := target.opts
	if o.Host != "jabber.example.org:5222" || o.User != "bot@example.org" || o.Password != "swordfish" || !o.InsecureSkipVerify {
		t.Errorf("the options are %+v", o)
	}
}

func TestSendMessagesDialsOnceAndPingsBeforeEachBatch(t *testing.T) {
	target, d := testTarget(t)
	ctx := context.Background()

	for _, body := range []string{"one", "two"} {
		if err := target.SendMessages(ctx, batch("alice@example.org", body)); err != nil {
			t.Fatal(err)
		}
	}

	if len(d.clients) != 1 {
		t.Fatalf("%d dials for two batches", len(d.clients))
	}
	c := d.clients[0]
	if c.pings != 1 || len(c.sent) != 2 || c.sent[1] != "alice@example.org: two" {
		t.Errorf("pings = %d, sent = %v, want one ping for the batch after the dial", c.pings, c.sent)
	}
}

func TestSendMessagesDialsAgainAfterAFailedPing(t *testing.T) {
	target, d := testTarget(t)
	ctx := context.Background()

	if err := target.SendMessages(ctx, batch("alice@example.org", "one")); err != nil {
		t.Fatal(err)
	}
	d.clients[0].pingErr = errs.ErrXMPPPingTimeout
	if err := target.SendMessages(ctx, batch("alice@example.org", "two")); err != nil {
		t.Fatal(err)
	}

	if len(d.clients) != 2 || !d.clients[0].closed || len(d.clients[1].sent) != 1 {
		t.Errorf("dials = %d, first closed = %v", len(d.clients), d.clients[0].closed)
	}
}

func TestSendMessagesDialsAgainWhenTheConnectionEnded(t *testing.T) {
	target, d := testTarget(t)
	ctx := context.Background()

	if err := target.SendMessages(ctx, batch("alice@example.org", "one")); err != nil {
		t.Fatal(err)
	}
	close(d.clients[0].done)
	if err := target.SendMessages(ctx, batch("alice@example.org", "two")); err != nil {
		t.Fatal(err)
	}

	if len(d.clients) != 2 || d.clients[0].pings != 0 || !d.clients[0].closed || len(d.clients[1].sent) != 1 {
		t.Errorf("dials = %d, pings on the dead client = %d, closed = %v", len(d.clients), d.clients[0].pings, d.clients[0].closed)
	}
}

func TestSendMessagesReturnsTheDialError(t *testing.T) {
	target, d := testTarget(t)
	d.err = errors.New("connection refused")

	if err := target.SendMessages(context.Background(), batch("alice@example.org", "one")); !errors.Is(err, d.err) {
		t.Errorf("SendMessages without a server = %v", err)
	}
	if target.client != nil {
		t.Error("a failed dial left a client behind")
	}
}

func TestSendMessagesStopsOnACancelledContext(t *testing.T) {
	target, d := testTarget(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := target.SendMessages(ctx, batch("alice@example.org", "one")); !errors.Is(err, context.Canceled) {
		t.Errorf("SendMessages under a cancelled context = %v", err)
	}
	if len(d.clients) != 0 {
		t.Error("a cancelled batch dialed")
	}
}

func TestOnMessageIsPassedToEveryDial(t *testing.T) {
	target, d := testTarget(t)
	target.OnMessage(func(xmppclient.Message) {})

	if err := target.Run(); err != nil {
		t.Fatal(err)
	}
	if len(d.opts) != 1 || d.opts[0].OnMessage == nil {
		t.Fatal("Run didn't dial with the callback")
	}
	if err := target.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if !d.clients[0].closed || target.client != nil {
		t.Error("Shutdown didn't close the client")
	}
}

func TestOnMessageAppliesToTheCurrentClient(t *testing.T) {
	target, d := testTarget(t)

	if err := target.Run(); err != nil {
		t.Fatal(err)
	}
	if d.opts[0].OnMessage != nil || d.clients[0].onMessage != nil {
		t.Fatal("a dial without a callback got one")
	}

	target.OnMessage(func(xmppclient.Message) {})
	if d.clients[0].onMessage == nil {
		t.Error("the callback wasn't set on the current client")
	}

	close(d.clients[0].done)
	if err := target.SendMessages(context.Background(), batch("alice@example.org", "one")); err != nil {
		t.Fatal(err)
	}
	if len(d.opts) != 2 || d.opts[1].OnMessage == nil {
		t.Error("the next dial went without the callback")
	}
}

func TestRunLogsAFailedDialAndRetriesAtTheFirstSend(t *testing.T) {
	target, d := testTarget(t)
	d.err = errors.New("connection refused")

	if err := target.Run(); err != nil {
		t.Errorf("Run with a server that refuses = %v", err)
	}
	d.err = nil
	if err := target.SendMessages(context.Background(), batch("alice@example.org", "one")); err != nil {
		t.Fatal(err)
	}
	if len(d.clients) != 1 || len(d.clients[0].sent) != 1 {
		t.Error("the first send didn't dial")
	}
}
