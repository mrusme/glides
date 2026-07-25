package runtime

import "testing"

func TestTrimTypeArgs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain method",
			in:   "xn--gckvb8fzb.com/glides/worker.(*Worker).HandleJob",
			want: "xn--gckvb8fzb.com/glides/worker.(*Worker).HandleJob",
		},
		{
			name: "generic function with a qualified type argument",
			in:   "xn--gckvb8fzb.com/glides/worker/targets/debug.HandlerFor[*xn--gckvb8fzb.com/app/models.Payload].func1",
			want: "xn--gckvb8fzb.com/glides/worker/targets/debug.HandlerFor.func1",
		},
		{
			name: "unterminated bracket",
			in:   "pkg.Fn[broken",
			want: "pkg.Fn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trimTypeArgs(tt.in); got != tt.want {
				t.Errorf("trimTypeArgs(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTrimClosures(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "single closure",
			in:   "pkg.HandlerFor.func1",
			want: "pkg.HandlerFor",
		},
		{
			name: "nested closures",
			in:   "pkg.HandlerFor.func1.func2",
			want: "pkg.HandlerFor",
		},
		{
			name: "method named func is kept",
			in:   "pkg.Func",
			want: "pkg.Func",
		},
		{
			name: "method with a func prefix is kept",
			in:   "pkg.functional",
			want: "pkg.functional",
		},
		{
			name: "no closure",
			in:   "pkg.(*Debug).SendMessages",
			want: "pkg.(*Debug).SendMessages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trimClosures(tt.in); got != tt.want {
				t.Errorf("trimClosures(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
