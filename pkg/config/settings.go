package config

import (
	"io/ioutil"
	"os"

	"github.com/juju/errors"
	"gopkg.in/yaml.v2"
)

var Settings SettingsRoot

type SettingsRoot struct {
	// Configuración de LetsEncrypt.
	ACME ACME `yaml:"acme"`

	// Configuración de dominios.
	Domains []Domain `yaml:"domains"`

	// Endpoint que tenemos que llamar para aplicar redirecciones.
	Redirects string `yaml:"redirects"`
}

type ACME struct {
	// Correo para registrar los certificados de LetsEncrypt.
	Email string `yaml:"email"`

	// Activa el flag para usar el servidor de pruebas en lugar del real.
	Staging bool `yaml:"staging"`
}

type Domain struct {
	// Nombre de esta entrada de dominio.
	Name string `yaml:"name"`

	// Nombre de dominio al que debemos responder.
	Hostname string `yaml:"hostname"`

	// Servicio de Kubernetes a donde mandamos las peticiones.
	Service string `yaml:"service"`

	// Rechaza los ficheros estáticos.
	RejectStaticAssets bool `yaml:"reject-static-assets"`

	// Si lo especificamos cambia la cabecera Host de las peticiones que redireccionamos.
	VirtualHostname string `yaml:"virtual-hostname"`

	// Enrutamiento individual de algunas direcciones dentro del dominio.
	Paths []Path `yaml:"paths"`

	// Configuración CORS del dominio.
	CORS CORS `yaml:"cors"`
}

type CORS struct {
	// Dominios de origen en los que está autorizado CORS.
	Origins []string `yaml:"origins"`
}

type Path struct {
	Match   string `yaml:"match"`
	Service string `yaml:"service"`
}

func ParseSettings() error {
	path := "/etc/baster/config.yml"
	if IsLocal() {
		path = "/etc/baster/config.dev.yml"
	}

	f, err := os.Open(path)
	if err != nil {
		return errors.Trace(err)
	}
	defer f.Close()

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return errors.Trace(err)
	}

	if err := yaml.Unmarshal(content, &Settings); err != nil {
		return errors.Trace(err)
	}

	return nil
}
