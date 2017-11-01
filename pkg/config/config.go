package config

import (
	"sync"

	"github.com/hashicorp/hcl"
	"github.com/juju/errors"

	"baster/pkg/external/k8s"
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

func (cnf *Config) NewController() *Controller {
	return &Controller{
		RWMutex: new(sync.RWMutex),
		cnf:     cnf,
	}
}

type ACME struct {
	Email string `hcl:"email"`
}

type Service struct {
	// Autofilled from the map key when loaded.
	Name string `hcl:"-"`

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

func Load(data string) (*Config, error) {
	cnf := new(Config)
	if err := hcl.Decode(cnf, data); err != nil {
		return nil, errors.Trace(err)
	}

	cnf.Version = cm.Metadata.ResourceVersion
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
