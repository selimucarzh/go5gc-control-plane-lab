package identity

import "testing"

func TestGenerate(t *testing.T) {
	ue := Generate(1, "001", "01", 42)

	if got, want := ue.IMSI, "001010000000042"; got != want {
		t.Fatalf("IMSI mismatch: got %s want %s", got, want)
	}

	if got, want := ue.SUPI, "imsi-001010000000042"; got != want {
		t.Fatalf("SUPI mismatch: got %s want %s", got, want)
	}
}
