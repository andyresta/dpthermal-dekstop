package printer

import "testing"

func TestEffectiveReceiptTextCols(t *testing.T) {
	tests := []struct {
		charsPerLine, size, want int
	}{
		{32, 1, 32},
		{32, 2, 16},
		{48, 2, 24},
		{32, 0, 32},
		{32, 99, 4},
	}
	for _, tc := range tests {
		got := effectiveReceiptTextCols(tc.charsPerLine, tc.size)
		if got != tc.want {
			t.Errorf("effectiveReceiptTextCols(%d, %d) = %d, want %d", tc.charsPerLine, tc.size, got, tc.want)
		}
	}
}

func TestPadReceiptTextLineCenter58mmSize2(t *testing.T) {
	got := padReceiptTextLine("NAMA TOKO", "center", 16)
	want := "   NAMA TOKO"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPadReceiptTextLineCenter80mmSize2(t *testing.T) {
	got := padReceiptTextLine("NAMA TOKO", "center", 24)
	want := "       NAMA TOKO"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPadReceiptTextLineRight(t *testing.T) {
	got := padReceiptTextLine("OK", "right", 16)
	want := "              OK"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPadReceiptTextLineLeftUnchanged(t *testing.T) {
	got := padReceiptTextLine("OK", "left", 16)
	if got != "OK" {
		t.Errorf("got %q, want %q", got, "OK")
	}
}

func TestPadReceiptTextLineOverflow(t *testing.T) {
	long := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	got := padReceiptTextLine(long, "center", 16)
	if got != long {
		t.Errorf("overflow should return line unchanged, got %q", got)
	}
}

func TestFormatReceiptTextDataLeftUnchanged(t *testing.T) {
	input := "NAMA TOKO"
	got := formatReceiptTextData(input, "left", 2, 32)
	if got != input {
		t.Errorf("got %q, want %q", got, input)
	}
}

func TestFormatReceiptTextDataMultiLine(t *testing.T) {
	got := formatReceiptTextData("A\nBB", "center", 1, 4)
	want := " A\n BB"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatReceiptTextDataTrailingNewline(t *testing.T) {
	got := formatReceiptTextData("OK\n", "center", 1, 4)
	want := " OK\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatReceiptTextData58mmSize2Center(t *testing.T) {
	got := formatReceiptTextData("NAMA TOKO", "center", 2, charsPerLineForPaper(58))
	want := "   NAMA TOKO"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatReceiptTextData80mmSize2Center(t *testing.T) {
	got := formatReceiptTextData("NAMA TOKO", "center", 2, charsPerLineForPaper(80))
	want := "       NAMA TOKO"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
