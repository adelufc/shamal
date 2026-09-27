package organization

import "testing"

func TestCloseBlockedWhileWorkRemains(t *testing.T) {
	if !CloseBlocked(1, 0, 0) || !CloseBlocked(0, 2, 0) || !CloseBlocked(0, 0, 3) {
		t.Fatal("active structure should block closing")
	}
	if CloseBlocked(0, 0, 0) {
		t.Fatal("empty node should close")
	}
}
