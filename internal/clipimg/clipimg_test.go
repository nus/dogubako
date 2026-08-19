package clipimg

import "testing"

func TestWritePNGEmpty(t *testing.T) {
	if err := WritePNG(nil); err == nil {
		t.Fatal("expected error")
	}
	if err := WritePNG([]byte{}); err == nil {
		t.Fatal("expected error")
	}
}
