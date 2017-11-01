
# baster

> Proxy inverso con LetsEncrypt e integrado con Kubernetes.

### Características

- Genera automáticamente los certificados conforme llega la primera petición.
- Usa el método TLS-SNI-02 de autenticación para ACME cuando es posible.
- Guarda los certificados en Datastore para una caché permanente sin necesidad de disco.
- Puede renovar los certificados sin necesidad de reiniciar nada y sin perder peticiones.
- Pueden configurarse distintas URLs apuntando a distintos servicios dentro de un mismo dominio.
- Carga la configuración desde un ConfigMap de Kubernetes.
- Detecta cambios en el ConfigMap y recarga la configuración sin necesidad de reiniciar.
- Soporte para HTTP/2 *out of the box*.
- Permite habilitar CORS a nivel de servicio para no tener que hacerlo en la aplicación.
- Health checks para que Kubernetes reinicie el contenedor si se quedara pillado.

### Configuración

El fichero de configuración está escrito en [HCL](https://github.com/hashicorp/hcl), un lenguaje inventado por Hashicorp para ser lo más simple posible.

La configuración consta de una sección ACME para configurar LetsEncrypt y uno o más servicios. Dentro de cada servicio se configura un `endpoint` que es el nombre del servicio de Kubernetes al que deben mandarse las peticiones.

```hcl
acme {
  email = "foo@example.com"
}

service "tipico-servicio" {
  endpoint = "tipico-servicio"
  hostname = "tipico-servicio.example.com"
}

service "servicio-con-cors" {
  endpoint = "foo"
  hostname = "foo.example.com"
  cors-enabled = true
}

service "servicio-con-http" {
  endpoint = "insecure"
  hostname = "insecure.example.com"
  allow-insecure = true
}
```


Cada servicio tiene una ruta por defecto que lo recoge todo y lo manda al endpoint general. Podemos especificar rutas nosotros mismos para personalizar el comportamiento aún más.

```hcl
acme {
  email = "foo@example.com"
}

service "personalizado" {
  endpoint = "personalizado"
  hostname = "personalizado.example.com"

  # Todo lo que empieze por "/accounts/login" permite acceder sin HTTP.
  route {
    url = "/accounts/login"
    allow-insecure = true
  }
  
  # Manda las peticiones que coincidan exactamente
  # con "/myaccount" ("/myaccount/foo" no entraría aquí) hacia
  # otro servicio distinto de Kubernetes.
  route {
    url = "/myaccount"
    endpoint = "myaccount-internal"
    exact-match = true
  }
}
```


#### Configuración de Kubernetes

El siguiente apartado contiene configuraciones de ejemplo para un despliegue de baster dentro de un cluster de Kubernetes en la nube (AWS o Google Cloud por ejemplo).

Servicio:

```yaml
kind: Service
apiVersion: v1
metadata:
  name: baster
spec:
  selector:
    app: baster
  ports:
  - port: 80
    targetPort: 80
    name: http
  - port: 443
    targetPort: 443
    name: https
  type: LoadBalancer
  loadBalancerIP: 1.2.3.4
```

Deployment:

```yaml
kind: Deployment
apiVersion: extensions/v1beta1
metadata:
  name: baster
spec:
  replicas: 2
  revisionHistoryLimit: 10
  strategy:
    rollingUpdate:
      maxUnavailable: 0
  template:
    metadata:
      labels:
        app: baster
    spec:
      containers:
      - name: baster
        image: eu.gcr.io/myapp/baster:latest
        volumeMounts:
        - name: config
          mountPath: /etc/baster
        ports:
        - containerPort: 80
          name: http
        - containerPort: 443
          name: https
        livenessProbe:
          httpGet:
            path: /health
            port: 80
          timeoutSeconds: 5
        readinessProbe:
          httpGet:
            path: /health
            port: 80
          timeoutSeconds: 5
        resources:
          requests:
            cpu: 10m
            memory: 200Mi
          limits:
            memory: 200Mi
      volumes:
      - name: config
        configMap:
          name: baster
```

Configmap:

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: baster
data:
  config.hcl:
    acme {
      email = "foo@example.com"
    }

    service "foo" {
      endpoint = "foo"
      hostname = "foo.example.com"
    }
```
