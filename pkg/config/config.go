package config

import (
	"github.com/hashicorp/hcl"
	"github.com/juju/errors"
)

type Config struct {
	// Autofilled from ConfigMap metadata when loaded.
	Version string `hcl:"-"`

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
	// Autofilled from the map key when loaded.
	Name string `hcl:"-"`

	Endpoint      string   `hcl:"endpoint"`
	Hostname      string   `hcl:"hostname"`
	Snakeoil      bool     `hcl:"snakeoil"`
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

func Load(version, data string) (*Config, error) {
	cnf := new(Config)
	if err := hcl.Decode(cnf, data); err != nil {
		return nil, errors.Trace(err)
	}

	cnf.Version = version
	for name, service := range cnf.Services {
		service.Name = name

		for _, route := range service.Routes {
			if !route.AllowInsecure && service.AllowInsecure {
				route.AllowInsecure = true
			}
			if route.Endpoint == "" {
				route.Endpoint = service.Endpoint
			}
		}
	}

	return cnf, nil
}
