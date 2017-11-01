package config

import (
	"github.com/juju/errors"
)

type Config struct {
	ACME     *ACME               `hcl:"acme"`
	Services map[string]*Service `hcl:"service"`
}

func (cnf *Config) IsValid() error {
	if cnf.ACME == nil || cnf.ACME.Email == "" {
		return errors.New("acme email required")
	}

	return nil
}

type ACME struct {
	Email string `hcl:"email"`
}

type Service struct {
	Endpoint      string   `hcl:"endpoint"`
	Hostname      string   `hcl:"hostname"`
	AllowInsecure bool     `hcl:"allow-insecure"`
	CORSEnabled   bool     `hcl:"cors-enabled"`
	Routes        []*Route `hcl:"route"`
}

type Route struct {
	URL           string `hcl:"url"`
	AllowInsecure bool   `hcl:"allow-insecure"`
	Endpoint      string `hcl:"endpoint"`
	ExactMatch    bool   `hcl:"exact-match"`
}
