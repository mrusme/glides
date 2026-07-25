package config

import "testing"

func TestTargetServes(t *testing.T) {
	tests := []struct {
		name    string
		target  Target
		channel string
		want    bool
	}{
		{
			name:    "email target serves email",
			target:  Target{ID: "notifications", Type: TargetTypeEmail},
			channel: TargetTypeEmail,
			want:    true,
		},
		{
			name:    "email target does not serve xmpp",
			target:  Target{ID: "notifications", Type: TargetTypeEmail},
			channel: TargetTypeXMPP,
			want:    false,
		},
		{
			name:    "xmpp target serves xmpp",
			target:  Target{ID: "jabber", Type: TargetTypeXMPP},
			channel: TargetTypeXMPP,
			want:    true,
		},
		{
			name: "debug target emulating email serves email",
			target: Target{ID: "debug-email", Type: TargetTypeDebug,
				Debug: TargetDebug{Emulates: TargetTypeEmail}},
			channel: TargetTypeEmail,
			want:    true,
		},
		{
			name: "debug target emulating email does not serve xmpp",
			target: Target{ID: "debug-email", Type: TargetTypeDebug,
				Debug: TargetDebug{Emulates: TargetTypeEmail}},
			channel: TargetTypeXMPP,
			want:    false,
		},
		{
			name: "debug target emulating xmpp serves xmpp",
			target: Target{ID: "debug-xmpp", Type: TargetTypeDebug,
				Debug: TargetDebug{Emulates: TargetTypeXMPP}},
			channel: TargetTypeXMPP,
			want:    true,
		},
		{
			name:    "debug target emulating nothing serves nothing",
			target:  Target{ID: "debug-broken", Type: TargetTypeDebug},
			channel: TargetTypeEmail,
			want:    false,
		},
		{
			name:    "debug target is never matched by its own type name",
			target:  Target{ID: "debug-email", Type: TargetTypeDebug, Debug: TargetDebug{Emulates: TargetTypeEmail}},
			channel: TargetTypeDebug,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.target.Serves(tt.channel); got != tt.want {
				t.Errorf("Serves(%q) = %v, want %v", tt.channel, got, tt.want)
			}
		})
	}
}
