package visits

import (
	"testing"
	"time"
)

func TestVisitMachineRejectsSkipsAndClosedStates(t *testing.T) {
	if !CanTransition(StatusNew, StatusAwaitingBroker) || !CanTransition(StatusAwaitingBroker, StatusAwaitingOwner) || !CanTransition(StatusAwaitingOwner, StatusConfirmed) {
		t.Fatal("broker then owner confirmation is the forward path")
	}
	if CanTransition(StatusAwaitingBroker, StatusConfirmed) {
		t.Fatal("broker confirmation cannot skip the owner")
	}
	if CanTransition(StatusCompleted, StatusConfirmed) || CanTransition(StatusCancelled, StatusAwaitingBroker) || CanTransition(StatusNoShow, StatusConfirmed) {
		t.Fatal("completed, cancelled, and missed visits are terminal")
	}
	if !CanTransition(StatusAwaitingOwner, StatusAwaitingBroker) {
		t.Fatal("owner rejection returns to the broker and is not a cancellation")
	}
	if !CanTransition(StatusConfirmed, StatusAwaitingBroker) {
		t.Fatal("reschedule is an event back to broker confirmation, not a terminal state")
	}
}

func TestSlotGapAndCompletion(t *testing.T) {
	base := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	aEnd := base.Add(DefaultDuration)
	near := aEnd.Add(10 * time.Minute)
	if !TooClose(base, aEnd, near, near.Add(DefaultDuration), SlotGap) {
		t.Fatal("ten minutes is inside the fifteen-minute gap")
	}
	far := aEnd.Add(20 * time.Minute)
	if TooClose(base, aEnd, far, far.Add(DefaultDuration), SlotGap) {
		t.Fatal("twenty minutes should stay clear")
	}
	if CompleteIssue(base, aEnd, "interested", "", false, "") != "too_early" {
		t.Fatal("completion before the end needs a manager correction")
	}
	if CompleteIssue(base, aEnd, "interested", "", true, "") != "reason_required" {
		t.Fatal("early correction needs a reason")
	}
	if CompleteIssue(aEnd, aEnd, "interested", "", false, "") != "" {
		t.Fatal("completion at the end is allowed")
	}
	if CompleteIssue(aEnd, aEnd, "bought", "", false, "") != "result_required" {
		t.Fatal("result must be one of the documented outcomes")
	}
}

func TestRemindersOnlyWhenTimeAllows(t *testing.T) {
	start := time.Date(2026, 10, 3, 16, 0, 0, 0, time.UTC)
	kinds := ReminderKinds(start.Add(-30*time.Hour), start)
	if len(kinds) != 2 {
		t.Fatalf("expected both reminders, got %v", kinds)
	}
	soon := ReminderKinds(start.Add(-90*time.Minute), start)
	if len(soon) != 0 {
		t.Fatalf("a near visit should not schedule reminders, got %v", soon)
	}
	if got := EndOrDefault(start, time.Time{}); !got.Equal(start.Add(DefaultDuration)) {
		t.Fatalf("default duration %v", got)
	}
}
