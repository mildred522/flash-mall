package handler

import (
	"encoding/json"
	"testing"
)

func TestParseUserIDClaim(t *testing.T) {
	cases := []struct {
		name string
		in   any
		ok   bool
		val  int64
	}{
		{name: "int64", in: int64(10), ok: true, val: 10},
		{name: "float64", in: float64(11), ok: true, val: 11},
		{name: "json number", in: json.Number("12"), ok: true, val: 12},
		{name: "string", in: "13", ok: true, val: 13},
		{name: "bad string", in: "abc", ok: false},
		{name: "nil", in: nil, ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseUserIDClaim(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok mismatch: got=%v want=%v", ok, tc.ok)
			}
			if ok && got != tc.val {
				t.Fatalf("value mismatch: got=%d want=%d", got, tc.val)
			}
		})
	}
}
