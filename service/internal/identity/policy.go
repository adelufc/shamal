package identity

import (
	"strings"
	"time"
	"unicode"
)

var CommonPasswords = []string{
	"password1234",
	"123456789012",
	"qwertyuiopas",
	"adminadmin12",
	"shamalshamal",
}

func PasswordAccepted(password string) bool {
	if len([]rune(password)) < 12 {
		return false
	}
	lower := strings.ToLower(password)
	for _, common := range CommonPasswords {
		if lower == common {
			return false
		}
	}
	hasLetter := false
	hasDigit := false
	for _, r := range password {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}

func NextLockout(attempts int, first *time.Time, now time.Time) (int, *time.Time, *time.Time) {
	window := 15 * time.Minute
	if first != nil && now.Sub(*first) > window {
		attempts = 0
		first = nil
	}
	attempts++
	if first == nil {
		t := now
		first = &t
	}
	if attempts >= 5 {
		until := now.Add(window)
		return attempts, first, &until
	}
	return attempts, first, nil
}

func SessionOpen(kind string, remember bool, created, lastSeen, now time.Time) bool {
	idle, absolute := 24*time.Hour, 24*time.Hour
	if kind == "staff" {
		idle, absolute = 30*time.Minute, 12*time.Hour
	} else if remember {
		absolute = 30 * 24 * time.Hour
	}
	if now.Sub(lastSeen) > idle {
		return false
	}
	if now.Sub(created) > absolute {
		return false
	}
	return true
}

func IdleTTL(kind string, remember bool, created, now time.Time) time.Duration {
	idle, absolute := 24*time.Hour, 24*time.Hour
	if kind == "staff" {
		idle, absolute = 30*time.Minute, 12*time.Hour
	} else if remember {
		absolute = 30 * 24 * time.Hour
	}
	remaining := absolute - now.Sub(created)
	if remaining < 0 {
		return 0
	}
	if remaining < idle {
		return remaining
	}
	return idle
}
