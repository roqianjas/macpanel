package system

import "testing"

func TestParsePurgeable(t *testing.T) {
	cases := []struct {
		out  string
		free uint64
		want uint64
	}{
		{"150285170929\n", 146044964864, 4240206065},
		{"146044964864", 146044964864, 0},
		{"100", 146044964864, 0},
		{"", 5, 0},
		{"abc", 5, 0},
		{"-5", 5, 0},
	}
	for _, c := range cases {
		if got := parsePurgeable(c.out, c.free); got != c.want {
			t.Errorf("%q free=%d: %d, want %d", c.out, c.free, got, c.want)
		}
	}
}
