package ice

import "errors"

// Typed errors
var (
	errSTUNInvalidMessage = errors.New("ice: STUN message is malformed")
)
