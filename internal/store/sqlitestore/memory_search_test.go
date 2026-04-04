//go:build sqlite || sqliteonly

package sqlitestore

import "testing"

func TestEscapeLike(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"hello", "hello"},
		{"50%", `50\%`},
		{"user_name", `user\_name`},
		{`back\slash`, `back\\slash`},
	}
	for _, tc := range cases {
		if got := escapeLike(tc.in); got != tc.want {
			t.Fatalf("escapeLike(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

