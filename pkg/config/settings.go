package config

import (
	"io/ioutil"
	"os"

	"github.com/hashicorp/hcl"
	"github.com/juju/errors"
)

type Settings struct {
	// Configuración de LetsEncrypt.
	ACME ACME `hcl:"acme"`

	// Configuración de dominios.
	Domains map[string]*Domain `hcl:"domain"`

	// Configuración para el servicio externo de redirecciones.
	Redirects Redirects `hcl:"redirects"`

	// Configuración de la monitorización.
	Monitoring Monitoring `hcl:"monitoring"`

	// Configuración para la autenticación.
	Auth Auth `hcl:"auth"`
}

type ACME struct {
	// Correo para registrar los certificados de LetsEncrypt.
	Email string `hcl:"email"`

	// Activa el flag para usar el servidor de pruebas en lugar del real.
	Staging bool `hcl:"staging"`
}

func (acme ACME) IsActive() bool {
	return acme.Email != ""
}

type Domain struct {
	// Nombre de esta entrada de dominio. Se autorellena desde la clave del mapa.
	Name string `hcl:"-"`

	// Hostname de la redirección. Si se especifica provocará una redirección automática
	// sin al otro dominio sin que llegue a ningún servicio de backend.
	Redirect string `hcl:"redirect"`

	// Nombre de dominio al que debemos responder.
	Hostname string `hcl:"hostname"`

	// Servicio de Kubernetes a donde mandamos las peticiones.
	Service string `hcl:"service"`

	// Si lo especificamos cambia la cabecera Host de las peticiones que redireccionamos.
	VirtualHostname string `hcl:"virtual-hostname"`

	// Enrutamiento individual de algunas direcciones dentro del dominio.
	Paths map[string]*Path `hcl:"path"`

	// Configuración CORS del dominio.
	CORS CORS `hcl:"cors"`

	// Mapa de cabeceras estáticas que mandaremos al servicio en las peticiones.
	// Se puede usar para sobreescribir la cabecera Host o cualquiera otra que necesitemos.
	// Si la asignamos a un valor vacío nos aseguramos que una cabecera nunca llege al servicio
	// mandada desde el cliente para mayor seguridad que necesite la aplicación.
	HopHeaders map[string]string `hcl:"hop-headers"`
}

type CORS struct {
	// Dominios de origen en los que está autorizado CORS.
	Origins []string `hcl:"origins"`
}

type Path struct {
	// Prefijo que se compara para ejecutar esta dirección. Se autorellena desde la clave del mapa.
	Match string `hcl:"-"`

	// Servicio que debe responder a las peticiones. Si está vacío se reusará
	// automáticamente el servicio que tengamos configurado a nivel de dominio.
	Service string `hcl:"service"`

	// Configuración de la monitorización para esta ruta concreta.
	Monitoring PathMonitoring `hcl:"monitoring"`

	// Etiquetas que debemos añadir a la monitorización cuando esta ruta se active.
	MonitoringTags map[string]string `hcl:"monitoring-tags"`
}

type PathMonitoring struct {
	// Nombre del servicio que usaremos para registrar el hit. Si no está especificado
	// usa el nombre que tenga asignado el dominio.
	Name string `hcl:"name"`

	// Etiquetas adicionales arbitrarias que podemos añadir a los hits para otros filtrados que queramos hacer.
	Tags map[string]string `hcl:"tags"`
}

type Redirects struct {
	// Endpoint externo que tenemos que llamar para aplicar redirecciones. Si está vacío
	// no se aplicarán redirecciones personalizadas.
	Apply string `hcl:"apply"`
}

type Monitoring struct {
	// Configuraciones para monitorizar las peticiones con BigQuery.
	BigQuery BigQuery `hcl:"bigquery"`
}

type BigQuery struct {
	// Nombre del dataset de BigQuery donde se deben insertar los datos.
	Dataset string `hcl:"dataset"`

	// Nombre de la tabla donde se deben insertar los datos.
	Table string `hcl:"table"`
}

func (bigquery *BigQuery) IsActive() bool {
	return bigquery.Dataset != "" && bigquery.Table != ""
}

type Auth struct {
	// Endpoint para la autenticación externa. Se llamará a cada petición para
	// saber si tenemos que rechazar la petición por permisos.
	Endpoint string `hcl:"endpoint"`
}

func ParseSettings() (*Settings, error) {
	path := "/etc/baster/config.hcl"
	if IsLocal() {
		path = "/etc/baster/config.dev.hcl"
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer f.Close()

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, errors.Trace(err)
	}

	settings := new(Settings)
	if err := hcl.Decode(settings, string(content)); err != nil {
		return nil, errors.Trace(err)
	}

	for name, domain := range settings.Domains {
		domain.Name = name

		for match, path := range domain.Paths {
			path.Match = match

			if path.Service == "" {
				path.Service = domain.Service
			}
		}
	}

	return settings, nil
}
