package identity

import (
	"testing"
	"time"
)

func TestPasswordPolicy(t *testing.T) {
	if PasswordAccepted("short1") {
		t.Fatal("short password accepted")
	}
	if PasswordAccepted("password1234") {
		t.Fatal("common password accepted")
	}
	if PasswordAccepted("longpassword") {
		t.Fatal("password without a digit accepted")
	}
	if !PasswordAccepted("Shamal-Test-12") {
		t.Fatal("expected the local test password to pass")
	}
}

func TestLockoutAfterFiveFailures(t *testing.T) {
	now := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	var first *time.Time
	var locked *time.Time
	attempts := 0
	for i := 0; i < 5; i++ {
		attempts, first, locked = NextLockout(attempts, first, now.Add(time.Duration(i)*time.Minute))
	}
	if locked == nil || attempts != 5 {
		t.Fatalf("attempts %d locked %v", attempts, locked)
	}
	later := now.Add(20 * time.Minute)
	attempts, _, locked = NextLockout(attempts, first, later)
	if attempts != 1 || locked != nil {
		t.Fatalf("window should reset, attempts %d locked %v", attempts, locked)
	}
}

func TestStaffSessionBounds(t *testing.T) {
	created := time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC)
	if !SessionOpen("staff", false, created, created.Add(20*time.Minute), created.Add(40*time.Minute)) {
		t.Fatal("active staff session rejected")
	}
	if SessionOpen("staff", false, created, created, created.Add(31*time.Minute)) {
		t.Fatal("idle staff session stayed open")
	}
	if SessionOpen("staff", false, created, created.Add(11*time.Hour), created.Add(13*time.Hour)) {
		t.Fatal("absolute staff session stayed open")
	}
	if !SessionOpen("client", true, created, created.Add(20*time.Hour), created.Add(23*time.Hour)) {
		t.Fatal("remembered client session rejected inside a day")
	}
	if SessionOpen("client", false, created, created.Add(23*time.Hour), created.Add(25*time.Hour)) {
		t.Fatal("client without remember exceeded a day")
	}
}
