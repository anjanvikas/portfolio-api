package service

import "testing"

func TestCheckSiteURLForUpload(t *testing.T) {
	cases := []struct {
		url   string
		allow bool
		bad   bool
	}{
		{"http://localhost:3000", false, true},
		{"http://127.0.0.1:3000", false, true},
		{"https://app.localhost", false, true},
		{"", false, true},
		{"http://localhost:3000", true, false},
		{"https://anjanvikasreddy.dev", false, false},
	}
	for _, c := range cases {
		err := CheckSiteURLForUpload(c.url, c.allow)
		if (err != nil) != c.bad {
			t.Errorf("%q allow=%v: err=%v want bad=%v", c.url, c.allow, err, c.bad)
		}
	}
}
