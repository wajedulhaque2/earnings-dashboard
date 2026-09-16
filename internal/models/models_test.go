package models

import "testing"

func TestSymbol(t *testing.T) {
	for _, v := range []string{" nvda ", "BRK-B", "7203.T", "^GSPC", "EURUSD=X"} {
		if _, err := Symbol(v); err != nil {
			t.Fatal(v, err)
		}
	}
	for _, v := range []string{"", "../etc", "A/B", "<script>", "A B", "A;DROP TABLE"} {
		if _, err := Symbol(v); err == nil {
			t.Fatal("accepted", v)
		}
	}
	v, _ := Symbol(" nvda ")
	if v != "NVDA" {
		t.Fatal(v)
	}
}
