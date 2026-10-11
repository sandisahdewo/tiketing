package money

import "testing"

func TestFormat(t *testing.T) {
	cases := map[int64]string{
		0:          "0",
		500:        "5",
		2000000:    "20.000",
		1250050:    "12.500,50",
		1250005:    "12.500,05",
		100000000:  "1.000.000",
		-1250050:   "-12.500,50",
		99:         "0,99",
		1234567800: "12.345.678",
	}
	for in, want := range cases {
		if got := Format(in); got != want {
			t.Errorf("Format(%d) = %q, want %q", in, got, want)
		}
	}
}
