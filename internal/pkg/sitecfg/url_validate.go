package sitecfg

import (
	"net/url"
	"strings"
)

func validateHTTPURL(s string) error {
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errBrandIconURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errBrandIconURL
	}
	if strings.Contains(u.Path, "..") {
		return errBrandIconPath
	}
	return nil
}
