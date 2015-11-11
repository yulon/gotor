package gotor

import (
	"net/http"
	"strings"
)

func IsMobile(r *http.Request) bool {
	ua := r.UserAgent()
	if strings.Index(ua, "Mobile") != -1 || strings.Index(ua, "Android") != -1 {
		return true
	}
	return false
}
