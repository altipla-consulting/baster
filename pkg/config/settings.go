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

	// Endpoint externo que tenemos que llamar para aplicar redirecciones. Si está vacío
	// no se aplicarán redirecciones personalizadas.
	Redirects string `yaml:"redirects"`

	// Configuración de la monitorización usando el stack TICK.
	Monitoring Monitoring `yaml:"monitoring"`
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

	// Si lo especificamos cambia la cabecera Host de las peticiones que redireccionamos.
	VirtualHostname string `yaml:"virtual-hostname"`

	// Enrutamiento individual de algunas direcciones dentro del dominio.
	Paths []Path `yaml:"paths"`

	// Configuración CORS del dominio.
	CORS CORS `yaml:"cors"`

	// Mapa de cabeceras estáticas que mandaremos al servicio en las peticiones.
	// Se puede usar para sobreescribir la cabecera Host o cualquiera otra que necesitemos.
	// Si la asignamos a un valor vacío nos aseguramos que una cabecera nunca llege al servicio
	// mandada desde el cliente para mayor seguridad que necesite la aplicación.
	HopHeaders map[string]string `yaml:"hop-headers"`
}

type CORS struct {
	// Dominios de origen en los que está autorizado CORS.
	Origins []string `yaml:"origins"`
}

type Path struct {
	// Prefijo que se compara para ejecutar esta dirección.
	Match string `yaml:"match"`

	// Servicio que debe responder a las peticiones. Si está vacío se reusará
	// automáticamente el servicio que tengamos configurado a nivel de dominio.
	Service string `yaml:"service"`

	// Etiquetas que debemos añadir a la monitorización cuando esta ruta se active.
	MonitoringTags map[string]string `yaml:"monitoring-tags"`
}

type Monitoring struct {
	// Dirección del servicio de monitorización. Si está vacío no se enviarán mediciones
	// de latencias a ningún servidor InfluxDB.
	Address string `yaml:"address"`

	// Nombre de usuario para mandar las mediciones de monitorización.
	Username string `yaml:"username"`

	// Contraseña para mandar las mediciones de monitorización.
	Password string `yaml:"password"`
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
