package care

import (
	"fmt"
	"unicode"
)

// ValidatePassword enforces a simple strength policy for the admin password:
// 8-20 characters, with at least one uppercase letter, one lowercase letter, and one
// number. Returns nil when acceptable, otherwise a human-readable reason to show.
func ValidatePassword(password string) error {
	n := len([]rune(password))
	if n < 8 {
		return fmt.Errorf("Password must be at least 8 characters.")
	}
	if n > 20 {
		return fmt.Errorf("Password must be 20 characters or fewer.")
	}
	var hasUpper, hasLower, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return fmt.Errorf("Password must include an uppercase letter, a lowercase letter, and a number.")
	}
	return nil
}
