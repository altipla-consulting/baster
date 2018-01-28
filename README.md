
# baster

> Proxy inverso con LetsEncrypt automático pensado para Kubernetes.


### Estado

**NOTA**: Este proxy está mantenido y es para uso interno (tiene poca documentación). Recomendamos al público general usar [Traefik](https://traefik.io/)


### Características

- Genera automáticamente los certificados conforme llega la primera petición.
- Guarda los certificados en Datastore para una caché permanente sin necesidad de disco.
- Puede renovar los certificados sin necesidad de reiniciar nada y sin perder peticiones.
- Pueden configurarse distintas URLs apuntando a distintos servicios dentro de un mismo dominio.
- Carga la configuración desde un ConfigMap de Kubernetes.
- Soporte para HTTP/2 *out of the box*.
- Health checks para que Kubernetes reinicie el contenedor si tiene problemas.


### Configuración

El fichero de configuración está escrito en YAML.

La configuración consta de una sección ACME para configurar LetsEncrypt y uno o más servicios. Dentro de cada servicio se configura un `endpoint` que es el nombre del servicio de Kubernetes al que deben mandarse las peticiones.

```yaml
acme:
  email: foo@example.com

domains:
- name: tipico-servicio
  hostname: tipico-servicio.example.com
  service: tipico-servicio:9000
```


Cada servicio tiene una ruta por defecto que lo recoge todo y lo manda al endpoint general. Podemos especificar rutas nosotros mismos para personalizar el comportamiento aún más.

```yaml

acme:
  email: foo@example.com

domains:
- name: personalizado
  hostname: personalizado.example.com
  paths:
  # Todo lo que empiece por /accounts/login lo manda al servicio de cuentas.
  - match: /accounts/login
    service: accounts:9000

  # El resto de peticiones las manda al servicio web (puerto 80).
  - match: /
    service: web
```


#### Configuración de Kubernetes

Tenemos una carpeta de ejemplo con configuraciones que con algunos mínimos cambios (el nombre del contenedor o la configuración) pueden desplegar baster dentro de un cluster de Kubernetes en Google Cloud.

La configuración de ejemplo incluye la cuenta de servicio que necesita baster para guardar los certificados en datastore.
