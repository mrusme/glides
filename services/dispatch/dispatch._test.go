package dispatch

import (
	"errors"
	"testing"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/services/config"
)

type stubResolver struct {
	routing Routing
	err     error
}

func (s stubResolver) Routing() (Routing, error) {
	return s.routing, s.err
}

func TestRoutingTargetIDFor(t *testing.T) {
	r := Routing{
		EmailTargetID: "notifications",
		XMPPTargetID:  "jabber",
	}

	tests := []struct {
		name  string
		isJID bool
		want  string
	}{
		{
			name:  "email recipient routes to the email target",
			isJID: false,
			want:  "notifications",
		},
		{
			name:  "jid recipient routes to the xmpp target",
			isJID: true,
			want:  "jabber",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.TargetIDFor(tt.isJID); got != tt.want {
				t.Errorf("TargetIDFor(%v) = %q, want %q", tt.isJID, got, tt.want)
			}
		})
	}
}

func TestRoutingUnconfiguredTargetsResolveEmpty(t *testing.T) {
	r := Routing{}

	for _, isJID := range []bool{false, true} {
		if got := r.TargetIDFor(isJID); got != "" {
			t.Errorf("TargetIDFor(%v) = %q, want empty", isJID, got)
		}
	}
}

func TestDebugTargetIDFor(t *testing.T) {
	disp, err := New(config.Redis{}, config.Targets{
		{ID: "notifications", Type: config.TargetTypeEmail},
		{ID: "jabber", Type: config.TargetTypeXMPP},
		{ID: "debug-email", Type: config.TargetTypeDebug,
			Debug: config.TargetDebug{Emulates: config.TargetTypeEmail}},
		{ID: "debug-xmpp", Type: config.TargetTypeDebug,
			Debug: config.TargetDebug{Emulates: config.TargetTypeXMPP}},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if got, want := disp.DebugTargetIDFor(config.TargetTypeEmail), "debug-email"; got != want {
		t.Errorf("DebugTargetIDFor(email) = %q, want %q", got, want)
	}
	if got, want := disp.DebugTargetIDFor(config.TargetTypeXMPP), "debug-xmpp"; got != want {
		t.Errorf("DebugTargetIDFor(xmpp) = %q, want %q", got, want)
	}
}

func TestDebugTargetIDForNeverPicksARealTarget(t *testing.T) {
	disp, err := New(config.Redis{}, config.Targets{
		{ID: "notifications", Type: config.TargetTypeEmail},
		{ID: "jabber", Type: config.TargetTypeXMPP},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if got := disp.DebugTargetIDFor(config.TargetTypeEmail); got != "" {
		t.Errorf("DebugTargetIDFor(email) = %q, want empty when no debug target is declared", got)
	}
	if got := disp.DebugTargetIDFor(config.TargetTypeXMPP); got != "" {
		t.Errorf("DebugTargetIDFor(xmpp) = %q, want empty when no debug target is declared", got)
	}
}

func TestDebugTargetIDForIgnoresMismatchedEmulation(t *testing.T) {
	disp, err := New(config.Redis{}, config.Targets{
		{ID: "debug-xmpp", Type: config.TargetTypeDebug,
			Debug: config.TargetDebug{Emulates: config.TargetTypeXMPP}},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if got := disp.DebugTargetIDFor(config.TargetTypeEmail); got != "" {
		t.Errorf("DebugTargetIDFor(email) = %q, want empty when only an xmpp debug target exists", got)
	}
}

func TestRoutingWithoutAResolverIsAnError(t *testing.T) {
	disp, err := New(config.Redis{}, config.Targets{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := disp.routing(); !errors.Is(err, errs.ErrDispatchResolverMissing) {
		t.Errorf("routing() error = %v, want %v", err, errs.ErrDispatchResolverMissing)
	}
}

func TestRoutingFallsBackToDebugTargetsPerChannel(t *testing.T) {
	disp, err := New(config.Redis{}, config.Targets{
		{ID: "debug-email", Type: config.TargetTypeDebug,
			Debug: config.TargetDebug{Emulates: config.TargetTypeEmail}},
		{ID: "debug-xmpp", Type: config.TargetTypeDebug,
			Debug: config.TargetDebug{Emulates: config.TargetTypeXMPP}},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	disp.SetResolver(stubResolver{routing: Routing{XMPPTargetID: "jabber"}})

	r, err := disp.routing()
	if err != nil {
		t.Fatalf("routing: %v", err)
	}

	if got, want := r.EmailTargetID, "debug-email"; got != want {
		t.Errorf("EmailTargetID = %q, want the debug target %q to fill an unset channel",
			got, want)
	}
	if got, want := r.XMPPTargetID, "jabber"; got != want {
		t.Errorf("XMPPTargetID = %q, want the resolved %q to survive the fallback",
			got, want)
	}
}

func TestRoutingPropagatesAResolverError(t *testing.T) {
	disp, err := New(config.Redis{}, config.Targets{
		{ID: "debug-email", Type: config.TargetTypeDebug,
			Debug: config.TargetDebug{Emulates: config.TargetTypeEmail}},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	want := errors.New("settings unavailable")
	disp.SetResolver(stubResolver{err: want})

	if _, err := disp.routing(); !errors.Is(err, want) {
		t.Errorf("routing() error = %v, want %v", err, want)
	}
}
