package auth

import "testing"

func TestDisplaySubscription(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"free", "Free"},
		{"pro", "Pro"},
		{"max", "Max"},
		{"team", "Team"},
		{"enterprise", "Enterprise"},
		{"some_new_plan", "some_new_plan"}, // raw passthrough
		{"", "Unknown"},
	}
	for _, tt := range tests {
		c := &Credentials{SubscriptionType: tt.raw}
		if got := c.DisplaySubscription(); got != tt.want {
			t.Errorf("DisplaySubscription(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestDisplayTier(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"default_claude_free", "Free"},
		{"default_claude_pro", "Pro"},
		{"default_claude_max_5x", "Max 5x"},
		{"default_claude_max_20x", "Max 20x"},
		{"default_claude_some_new_tier", "default_claude_some_new_tier"}, // raw passthrough
		{"", "Unknown"},
	}
	for _, tt := range tests {
		c := &Credentials{RateLimitTier: tt.raw}
		if got := c.DisplayTier(); got != tt.want {
			t.Errorf("DisplayTier(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}
