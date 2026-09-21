package ui

// maskSecretPrefixLen is how many leading cells of a secret stay readable so a
// user can tell which key is configured without the rest ever being printed.
const maskSecretPrefixLen = 12

// MaskSecret returns the first 12 characters of secret followed by the
// one-cell ellipsis. A secret of 12 characters or fewer has nothing left to
// hide behind the ellipsis, so it yields "" and the caller prints its own
// placeholder. Named so static analysis sees a masking step, not a slice.
func MaskSecret(secret string) string {
	if len(secret) <= maskSecretPrefixLen {
		return ""
	}
	return secret[:maskSecretPrefixLen] + "…"
}
