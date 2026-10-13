package xmppclient

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	goxmpp "github.com/xmppo/go-xmpp"
	"xn--gckvb8fzb.com/glides/errs"
)

type fakeSession struct {
	in     chan any
	writes chan string
	closed chan struct{}
	once   sync.Once
}

func newFakeSession() *fakeSession {
	return &fakeSession{
		in:     make(chan any, 16),
		writes: make(chan string, 64),
		closed: make(chan struct{}),
	}
}

func (f *fakeSession) Recv() (any, error) {
	select {
	case stanza := <-f.in:
		if err, ok := stanza.(error); ok {
			return nil, err
		}
		return stanza, nil
	case <-f.closed:
		return nil, io.EOF
	}
}

func (f *fakeSession) Send(chat goxmpp.Chat) (int, error) {
	f.writes <- "chat " + chat.Remote + " " + chat.Text
	return 0, nil
}

func (f *fakeSession) SendOrg(stanza string) (int, error) {
	f.writes <- stanza
	if strings.Contains(stanza, "</stream:stream>") {
		f.in <- errors.New("server closed stream")
	}
	return len(stanza), nil
}

func (f *fakeSession) Close() error {
	f.once.Do(func() { close(f.closed) })
	return nil
}

func (f *fakeSession) JID() string {
	return "bot@example.org/test"
}

func (f *fakeSession) isClosed() bool {
	select {
	case <-f.closed:
		return true
	default:
		return false
	}
}

func idOf(stanza string) string {
	_, after, ok := strings.Cut(stanza, "id='")
	if !ok {
		return ""
	}
	id, _, _ := strings.Cut(after, "'")
	return id
}

func (f *fakeSession) answerPings(stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case w := <-f.writes:
			if strings.Contains(w, "urn:xmpp:ping") {
				f.in <- goxmpp.IQ{Type: "result", ID: idOf(w)}
			}
		}
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func testLog() (*slog.Logger, *syncBuffer) {
	buf := new(syncBuffer)
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})), buf
}

func waitFor(t *testing.T, buf *syncBuffer, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the log never contained %q:\n%s", want, buf.String())
}

func TestServerNameAndDomain(t *testing.T) {
	for in, want := range map[string]string{
		"jabber.example.org:5222": "jabber.example.org",
		"jabber.example.org":      "jabber.example.org",
		"[2001:db8::1]:5222":      "2001:db8::1",
	} {
		if got := serverName(in); got != want {
			t.Errorf("serverName(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"bot@example.org/test": "example.org",
		"bot@example.org":      "example.org",
		"example.org":          "example.org",
	} {
		if got := domainOf(in); got != want {
			t.Errorf("domainOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPingMatchesItsReply(t *testing.T) {
	f := newFakeSession()
	c := start(nil, Options{PingTimeout: time.Second, PingInterval: time.Hour}, f)
	defer c.Close()

	go func() {
		w := <-f.writes
		f.in <- goxmpp.IQ{Type: "result", ID: "stale"}
		f.in <- goxmpp.IQ{Type: "result", ID: idOf(w)}
	}()

	if err := c.Ping(); err != nil {
		t.Errorf("Ping = %v", err)
	}
	if c.JID() != "bot@example.org/test" || c.domain != "example.org" {
		t.Errorf("JID = %q, domain = %q", c.JID(), c.domain)
	}
}

func TestPingTimesOutWithoutAReply(t *testing.T) {
	f := newFakeSession()
	c := start(nil, Options{PingTimeout: 30 * time.Millisecond, PingInterval: time.Hour}, f)
	defer c.Close()

	if err := c.Ping(); !errors.Is(err, errs.ErrXMPPPingTimeout) {
		t.Errorf("Ping without a reply = %v, want %v", err, errs.ErrXMPPPingTimeout)
	}
	if w := <-f.writes; !strings.Contains(w, "<ping xmlns='urn:xmpp:ping'/>") || !strings.Contains(w, "to='example.org'") {
		t.Errorf("the ping written was %q", w)
	}
}

func TestMessagesReachTheCallbackOrTheLog(t *testing.T) {
	log, buf := testLog()
	got := make(chan Message, 1)
	f := newFakeSession()
	c := start(log, Options{PingInterval: time.Hour, OnMessage: func(m Message) { got <- m }}, f)
	defer c.Close()

	f.in <- goxmpp.Chat{Type: "chat", Remote: "alice@example.org/phone", Text: "hello"}
	select {
	case m := <-got:
		if m.From != "alice@example.org/phone" || m.Body != "hello" {
			t.Errorf("the callback got %+v", m)
		}
	case <-time.After(time.Second):
		t.Fatal("the callback never ran")
	}

	f.in <- goxmpp.Chat{Type: "error", Remote: "nobody@example.org", OtherElem: []goxmpp.XMLElement{{
		XMLName:  xml.Name{Space: "jabber:client", Local: "error"},
		Attr:     []xml.Attr{{Name: xml.Name{Local: "type"}, Value: "cancel"}},
		InnerXML: `<service-unavailable xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/>`,
	}}}
	waitFor(t, buf, "message rejected")
	if s := buf.String(); !strings.Contains(s, "condition=service-unavailable") || !strings.Contains(s, "remote=nobody@example.org") {
		t.Errorf("the rejection was logged as:\n%s", s)
	}

	f.in <- goxmpp.Chat{Type: "chat", Remote: "alice@example.org/phone", Text: ""}
	f.in <- goxmpp.Presence{From: "alice@example.org/phone", Type: "subscribe"}
	waitFor(t, buf, "status=presence")
	if len(got) != 0 {
		t.Error("a message without a body reached the callback")
	}

	log2, buf2 := testLog()
	f2 := newFakeSession()
	c2 := start(log2, Options{PingInterval: time.Hour}, f2)
	defer c2.Close()
	f2.in <- goxmpp.Chat{Type: "normal", Remote: "alice@example.org", Text: "hello"}
	waitFor(t, buf2, "message dropped")
	if s := buf2.String(); !strings.Contains(s, "bytes=5") || strings.Contains(s, "hello") {
		t.Errorf("the dropped message was logged as:\n%s", s)
	}
}

func TestAStreamEndClosesTheSessionAndDone(t *testing.T) {
	log, buf := testLog()
	f := newFakeSession()
	c := start(log, Options{PingInterval: time.Hour}, f)

	f.in <- io.ErrUnexpectedEOF
	select {
	case <-c.Done():
	case <-time.After(time.Second):
		t.Fatal("Done wasn't closed after the stream ended")
	}
	if !f.isClosed() {
		t.Error("the session wasn't closed")
	}
	if err := c.Send("alice@example.org", "x"); !errors.Is(err, errs.ErrXMPPClosed) {
		t.Errorf("Send after the end = %v, want %v", err, errs.ErrXMPPClosed)
	}
	if err := c.Ping(); !errors.Is(err, errs.ErrXMPPClosed) {
		t.Errorf("Ping after the end = %v, want %v", err, errs.ErrXMPPClosed)
	}
	if !strings.Contains(buf.String(), "level=INFO") || !strings.Contains(buf.String(), "stream ended") {
		t.Errorf("the end was logged as:\n%s", buf.String())
	}

	started := time.Now()
	if err := c.Close(); err != nil || time.Since(started) > time.Second {
		t.Errorf("Close after the end = %v after %v", err, time.Since(started))
	}
}

func TestCloseSendsTheStreamEndAndWaits(t *testing.T) {
	log, buf := testLog()
	f := newFakeSession()
	c := start(log, Options{PingInterval: time.Hour}, f)

	started := time.Now()
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) > time.Second {
		t.Errorf("Close took %v", time.Since(started))
	}
	select {
	case <-c.Done():
	default:
		t.Error("Done isn't closed after Close")
	}
	if !f.isClosed() {
		t.Error("the session wasn't closed")
	}

	found := false
	for len(f.writes) > 0 {
		if strings.Contains(<-f.writes, "</stream:stream>") {
			found = true
		}
	}
	if !found {
		t.Error("the stream end wasn't written")
	}
	if s := buf.String(); strings.Contains(s, "level=INFO") || strings.Contains(s, "level=WARN") {
		t.Errorf("a clean close logged above debug:\n%s", s)
	}

	if err := c.Close(); err != nil {
		t.Errorf("a second Close = %v", err)
	}
}

func TestTheTickerClosesAfterAnUnansweredPing(t *testing.T) {
	log, buf := testLog()
	f := newFakeSession()
	c := start(log, Options{PingInterval: 20 * time.Millisecond, PingTimeout: 20 * time.Millisecond}, f)

	select {
	case <-c.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the client stayed open without a ping reply")
	}
	waitFor(t, buf, "no reply, closing")
	if !f.isClosed() {
		t.Error("the session wasn't closed")
	}
}

func TestTheTickerKeepsAnAnsweredConnection(t *testing.T) {
	f := newFakeSession()
	stop := make(chan struct{})
	defer close(stop)
	go f.answerPings(stop)

	c := start(nil, Options{PingInterval: 20 * time.Millisecond, PingTimeout: 500 * time.Millisecond}, f)
	defer c.Close()

	select {
	case <-c.Done():
		t.Error("the client closed although every ping was answered")
	case <-time.After(200 * time.Millisecond):
	}
	if err := c.Send("alice@example.org", "still here"); err != nil {
		t.Errorf("Send = %v", err)
	}
}

func TestACallbackMaySendThroughTheClient(t *testing.T) {
	f := newFakeSession()
	var c *Client
	replied := make(chan error, 1)
	c = start(nil, Options{
		PingTimeout:  time.Second,
		PingInterval: time.Hour,
		OnMessage:    func(m Message) { replied <- c.Send(m.From, "reply") },
	}, f)
	defer c.Close()

	go func() {
		w := <-f.writes
		time.Sleep(50 * time.Millisecond)
		f.in <- goxmpp.Chat{Type: "chat", Remote: "alice@example.org", Text: "hi"}
		time.Sleep(50 * time.Millisecond)
		f.in <- goxmpp.IQ{Type: "result", ID: idOf(w)}
	}()

	started := time.Now()
	if err := c.Ping(); err != nil || time.Since(started) > 500*time.Millisecond {
		t.Errorf("Ping while a callback sends = %v after %v", err, time.Since(started))
	}
	select {
	case err := <-replied:
		if err != nil {
			t.Errorf("the callback's Send = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("the callback never finished")
	}
	if w := <-f.writes; w != "chat alice@example.org reply" {
		t.Errorf("the reply written was %q", w)
	}
}

func TestSetOnMessageAppliesToTheRunningConnection(t *testing.T) {
	log, buf := testLog()
	got := make(chan Message, 1)
	f := newFakeSession()
	c := start(log, Options{PingInterval: time.Hour}, f)
	defer c.Close()

	f.in <- goxmpp.Chat{Type: "chat", Remote: "alice@example.org", Text: "early"}
	waitFor(t, buf, "message dropped")

	c.SetOnMessage(func(m Message) { got <- m })
	f.in <- goxmpp.Chat{Type: "chat", Remote: "alice@example.org", Text: "late"}
	select {
	case m := <-got:
		if m.Body != "late" {
			t.Errorf("the callback got %+v", m)
		}
	case <-time.After(time.Second):
		t.Fatal("a callback set on the running connection wasn't used")
	}

	c.SetOnMessage(nil)
	f.in <- goxmpp.Chat{Type: "chat", Remote: "alice@example.org", Text: "unset"}
	waitFor(t, buf, "bytes=5")
	if len(got) != 0 {
		t.Error("a removed callback was used")
	}
}

func TestMessagesAreDeliveredInOrderAndASlowCallbackDropsWithAWarning(t *testing.T) {
	log, buf := testLog()
	release := make(chan struct{})
	var mu sync.Mutex
	var order []string
	f := newFakeSession()
	c := start(log, Options{PingInterval: time.Hour, PingTimeout: time.Second, OnMessage: func(m Message) {
		<-release
		mu.Lock()
		order = append(order, m.Body)
		mu.Unlock()
	}}, f)
	defer c.Close()

	for i := 0; i <= QueueLength+1; i++ {
		f.in <- goxmpp.Chat{Type: "chat", Remote: "alice@example.org", Text: fmt.Sprintf("%03d", i)}
	}
	waitFor(t, buf, "the callback is behind")

	go func() {
		w := <-f.writes
		f.in <- goxmpp.IQ{Type: "result", ID: idOf(w)}
	}()
	if err := c.Ping(); err != nil {
		t.Errorf("Ping while the callback is behind = %v", err)
	}

	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(order)
		mu.Unlock()
		if n == QueueLength+1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(order) != QueueLength+1 || order[0] != "000" || order[QueueLength] != fmt.Sprintf("%03d", QueueLength) {
		t.Errorf("%d messages delivered, first %q, last %q", len(order), order[0], order[len(order)-1])
	}
	if !slices.IsSorted(order) {
		t.Error("the messages arrived out of order")
	}
}
