package xmpp

import "testing"

func TestServerName(t *testing.T) {
	for in, want := range map[string]string{
		"jabber.example.org:5222": "jabber.example.org",
		"jabber.example.org":      "jabber.example.org",
		"[2001:db8::1]:5222":      "2001:db8::1",
	} {
		if got := serverName(in); got != want {
			t.Errorf("serverName(%q) = %q, want %q", in, got, want)
		}
	}
}
