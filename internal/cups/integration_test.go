package cups

import (
	"context"
	"errors"
	"testing"
)

// TestAgainstLocalCUPS talks to the CUPS on this machine. It skips itself when
// the daemon is not running, so the suite still passes where CUPS is absent.
func TestAgainstLocalCUPS(t *testing.T) {
	if testing.Short() {
		t.Skip("needs a live CUPS")
	}
	c, err := New()
	if err != nil {
		t.Fatal(err)
	}
	snap, err := c.Snapshot(context.Background())
	var cerr *Error
	if errors.As(err, &cerr) && cerr.Kind == KindDaemonDown {
		t.Skipf("CUPS is not running: %v", err)
	}
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	t.Logf("default=%q printers=%d jobs=%d", snap.Default, len(snap.Printers), len(snap.Jobs))
	for _, p := range snap.Printers {
		t.Logf("  %s state=%v accepting=%v default=%v reasons=%v model=%q",
			p.Name, p.State, p.Accepting, p.IsDefault, p.Reasons, p.MakeModel)
	}
}
