package cups

import (
	"context"
	"errors"
	"testing"
)

// TestAgainstLocalCUPS habla con el CUPS de la máquina. Se saltea solo si el
// demonio no está corriendo, así no rompe la suite en un entorno sin CUPS.
func TestAgainstLocalCUPS(t *testing.T) {
	if testing.Short() {
		t.Skip("necesita un CUPS vivo")
	}
	c, err := New()
	if err != nil {
		t.Fatal(err)
	}
	snap, err := c.Snapshot(context.Background())
	var cerr *Error
	if errors.As(err, &cerr) && cerr.Kind == KindDaemonDown {
		t.Skipf("CUPS no está corriendo: %v", err)
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
