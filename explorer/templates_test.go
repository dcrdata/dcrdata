package explorer

import (
	"testing"
)

func TestAmountAsDecimalPartsTrimmed(t *testing.T) {

	in := []struct {
		amt    int64
		n      int64
		commas bool
	}{
		{314159000, 2, false},
		{76543210000, 2, false},
		{766432100000, 2, true},
		{654321000, 1, false},
		{987654321, 8, false},
		{987654321, 2, false},
		{90765432100, 2, false},
		{9076543200000, 2, false},
		{907654320, 7, false},
		{1234590700, 2, false},
		{100000000, 2, false},
		{314159000, 2, false},
		{14159000, 2, false},
		{314159000, 7, true},
		{300000000, 7, true},
		{301000000, 1, true},
		{300000000, 0, true},
		{987654321, 11, false},
		{987654321237, 11, false},
	}

	expected := []struct {
		whole, frac, tail string
	}{
		{"3", "14", ""},
		{"765", "43", ""},
		{"7,664", "32", ""},
		{"6", "5", ""},
		{"9", "87654321", ""},
		{"9", "87", ""},
		{"907", "65", ""},
		{"90765", "43", ""},
		{"9", "0765432", ""},
		{"12", "34", ""},
		{"1", "", "00"},
		{"3", "14", ""},
		{"0", "14", ""},
		{"3", "14159", "00"},
		{"3", "", "0000000"},
		{"3", "", "0"},
		{"3", "", ""},
		{"9", "87654321", ""},
		{"9876", "54321237", ""},
	}

	for i := range in {
		out := amountAsDecimalPartsTrimmed(in[i].amt, in[i].n, in[i].commas)
		if out[0] != expected[i].whole || out[1] != expected[i].frac ||
			out[2] != expected[i].tail {
			t.Errorf("amountAsDecimalPartsTrimmed failed for "+
				"%d (%d decimals, commas=%v). Got %s.%s%s, expected %s.%s%s.",
				in[i].amt, in[i].n, in[i].commas,
				out[0], out[1], out[2],
				expected[i].whole, expected[i].frac, expected[i].tail)
		}

	}

}
