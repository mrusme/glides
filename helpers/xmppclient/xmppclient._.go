package xmppclient

import (
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	goxmpp "github.com/xmppo/go-xmpp"
	"xn--gckvb8fzb.com/glides/errs"
)

const (
	DefaultDialTimeout  time.Duration = 10 * time.Second
	DefaultPingTimeout  time.Duration = 5 * time.Second
	DefaultPingInterval time.Duration = 60 * time.Second
	CloseTimeout        time.Duration = 10 * time.Second
	QueueLength         int           = 256
)

type Options struct {
	Host               string
	User               string
	Password           string
	ServerName         string
	InsecureSkipVerify bool
	DirectTLS          bool
	Status             string
	Debug              bool
	DialTimeout        time.Duration
	PingTimeout        time.Duration
	PingInterval       time.Duration
	OnMessage          func(Message)
}

type Message struct {
	From string
	Body string
}

type session interface {
	Recv() (any, error)
	Send(goxmpp.Chat) (int, error)
	SendOrg(string) (int, error)
	Close() error
	JID() string
}

type Client struct {
	log     *slog.Logger
	opts    Options
	session session
	jid     string
	domain  string

	mu sync.Mutex

	pingMu  sync.Mutex
	pending string
	pong    chan string

	onMessage atomic.Pointer[func(Message)]
	messages  chan Message
	delivered chan struct{}

	done      chan struct{}
	closing   chan struct{}
	closeOnce sync.Once
}

func (o Options) withDefaults() Options {
	if o.ServerName == "" {
		o.ServerName = serverName(o.Host)
	}
	if o.Status == "" {
		o.Status = "chat"
	}
	if o.DialTimeout == 0 {
		o.DialTimeout = DefaultDialTimeout
	}
	if o.PingTimeout == 0 {
		o.PingTimeout = DefaultPingTimeout
	}
	if o.PingInterval == 0 {
		o.PingInterval = DefaultPingInterval
	}

	return o
}

func serverName(host string) string {
	if name, _, err := net.SplitHostPort(host); err == nil {
		return name
	}

	return host
}

func domainOf(jid string) string {
	if at := strings.IndexByte(jid, '@'); at >= 0 {
		jid = jid[at+1:]
	}
	if slash := strings.IndexByte(jid, '/'); slash >= 0 {
		jid = jid[:slash]
	}

	return jid
}

func Dial(log *slog.Logger, opts Options) (*Client, error) {
	opts = opts.withDefaults()

	s, err := goxmpp.Options{
		Host:     opts.Host,
		User:     opts.User,
		Password: opts.Password,
		NoTLS:    !opts.DirectTLS,
		StartTLS: !opts.DirectTLS,
		TLSConfig: &tls.Config{
			ServerName:         opts.ServerName,
			InsecureSkipVerify: opts.InsecureSkipVerify,
		},
		Debug:       opts.Debug,
		Session:     true,
		Status:      opts.Status,
		DialTimeout: opts.DialTimeout,
	}.NewClient()
	if err != nil {
		return nil, err
	}

	return start(log, opts, s), nil
}

func start(log *slog.Logger, opts Options, s session) *Client {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	c := &Client{
		log:       log,
		opts:      opts.withDefaults(),
		session:   s,
		jid:       s.JID(),
		pong:      make(chan string, 1),
		messages:  make(chan Message, QueueLength),
		delivered: make(chan struct{}),
		done:      make(chan struct{}),
		closing:   make(chan struct{}),
	}
	c.domain = domainOf(c.jid)
	c.SetOnMessage(opts.OnMessage)

	go c.receive()
	go c.deliver()
	go c.keep()

	return c
}

func (c *Client) SetOnMessage(fn func(Message)) {
	if fn == nil {
		c.onMessage.Store(nil)
		return
	}

	c.onMessage.Store(&fn)
}

func (c *Client) JID() string {
	return c.jid
}

func (c *Client) Done() <-chan struct{} {
	return c.done
}

func (c *Client) closed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *Client) Send(to string, body string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed() {
		return errs.ErrXMPPClosed
	}

	_, err := c.session.Send(goxmpp.Chat{
		Remote: to,
		Type:   "chat",
		Text:   body,
	})

	return err
}

func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		close(c.closing)

		c.mu.Lock()
		if !c.closed() {
			if _, err := c.session.SendOrg("</stream:stream>"); err != nil {
				c.log.Debug("XMPPClient.Close", "status", "stream end not sent", "error", err)
			}
		}
		c.mu.Unlock()

		select {
		case <-c.done:
		case <-time.After(CloseTimeout):
			c.log.Warn("XMPPClient.Close", "status", "no stream end from the server, closing the socket")
			c.session.Close()
			<-c.done
		}
	})

	return nil
}
