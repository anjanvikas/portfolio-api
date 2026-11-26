package service

import (
	"fmt"
	"net/url"
	"strings"
)

// CheckSiteURLForUpload refuses a localhost SITE_URL for runs that write OG
// cards to shared storage (the host is baked into the card graphic). Pass
// allowLocal to override deliberately.
func CheckSiteURLForUpload(siteURL string, allowLocal bool) error {
	if allowLocal {
		return nil
	}
	h := siteHost(siteURL)
	host := h
	if u, err := url.Parse("//" + h); err == nil {
		host = u.Hostname()
	}
	host = strings.ToLower(host)
	if host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("SITE_URL %q is local; set SITE_URL to the real origin or pass -allow-localhost", siteURL)
	}
	return nil
}
