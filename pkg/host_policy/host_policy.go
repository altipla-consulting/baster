package host_policy

import (
	"golang.org/x/crypto/acme/autocert"

	"github.com/altipla-consulting/baster/pkg/config"
)

func New() autocert.HostPolicy {
	var whitelist []string
	for _, domain := range config.Settings.Domains {
		whitelist = append(whitelist, domain.Hostname)
	}

	return autocert.HostWhitelist(whitelist...)
}
