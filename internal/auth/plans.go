package auth

// subscriptionNames maps subscriptionType machine values to the plan names used
// on the Anthropic pricing page (https://www.claude.com/pricing).
var subscriptionNames = map[string]string{
	"free":       "Free",
	"pro":        "Pro",
	"max":        "Max",
	"team":       "Team",
	"enterprise": "Enterprise",
}

// tierNames maps rateLimitTier machine values to the corresponding plan names.
var tierNames = map[string]string{
	"default_claude_free":    "Free",
	"default_claude_pro":     "Pro",
	"default_claude_max_5x":  "Max 5x",
	"default_claude_max_20x": "Max 20x",
}

// DisplaySubscription returns the human-readable plan name for the subscription
// type, falling back to the raw value, then "Unknown" when empty.
func (c *Credentials) DisplaySubscription() string {
	return displayName(c.SubscriptionType, subscriptionNames)
}

// DisplayTier returns the human-readable plan name for the rate-limit tier,
// using the same fallback chain as DisplaySubscription.
func (c *Credentials) DisplayTier() string {
	return displayName(c.RateLimitTier, tierNames)
}

// displayName looks raw up in m, returning the mapped name, the raw value when
// unmapped (so new Anthropic plans still surface), or "Unknown" when empty.
func displayName(raw string, m map[string]string) string {
	if raw == "" {
		return "Unknown"
	}
	if name, ok := m[raw]; ok {
		return name
	}
	return raw
}
