package nginx

import (
	"text/template"
	"bytes"

	"baster/pkg/config"
)

var configTemplate = template.MustCompile(`
	user www-data;
	worker_processes 4;
	pid /var/run/nginx.pid;

	events {
	  worker_connections 1024;
	}

	http {
	  tcp_nopush on;
	  tcp_nodelay on;
	  types_hash_max_size 2048;

	  include /etc/nginx/mime.types;
	  default_type application/octet-stream;

	  # Custom access log format.
	  log_format altipla 'time:[$time_local] host:[$host] latency:[$request_time] method:[$request_method] referer:[$http_referer] request-size:[$request_length] response-size:[$bytes_sent] scheme:[$scheme] status:[$status] uri:[$uri] user-agent:[$http_user_agent] auth:[$http_authentication] service:[$proxy_host]';
	  access_log /dev/stdout altipla;

	  # Error logs received by Kubernetes.
	  error_log stderr;

	  gzip on;
	  gzip_disable "msie6";

	  # Enable gzip even in requests with the Via proxy header.
	  gzip_proxied any;
	  
	  # Tell the proxies to cache depending on the Accept-Encoding header of the request.
	  gzip_vary on;

	  {{.Services}}
	  
	  resolver {{.DNSIP}} valid=5m;
	  resolver_timeout 10s;
	}
`)

var servicesTemplate = template.MustCompile(`
	{{range .}}
		server {
			listen 80;
			include errors.conf;
			include acme.conf;
			server_name {{.Hostname}};

			{{range .Routes}}
				{{if .AllowInsecure}}
					location {{if .ExactMatch}}={{end}} {{.URL}} {
						set $backend_upstream "http://{{.Endpoint}}.default.svc.cluster.local";
						proxy_pass $backend_upstream;

						proxy_redirect off;
						proxy_set_header Host $host;
						proxy_connect_timeout 10s;
					}
				{{end}}
			{{end}}

			{{if .AllowInsecure}}
				location / {
					set $backend_upstream "http://{{.Endpoint}}.default.svc.cluster.local";
					proxy_pass $backend_upstream;

					proxy_redirect off;
					proxy_set_header Host $host;
					proxy_connect_timeout 10s;
				}
			{{else}}
				location / {
			    return 301 https://{{.Hostname}}$request_uri;		
				}
			{{end}}
		}

		server {
			listen 443;
			include errors.conf;
			server_name {{.Hostname}};

		  ssl_certificate /etc/certificates/{{.Name}}.crt;
		  ssl_certificate_key /etc/certificates/{{.Name}}.key;

			{{range .Routes}}
				location {{if .ExactMatch}}={{end}} {{.URL}} {
					set $backend_upstream "http://{{.Endpoint}}.default.svc.cluster.local";
					proxy_pass $backend_upstream;

					proxy_redirect off;
					proxy_set_header Host $host;
					proxy_connect_timeout 10s;
				}
			{{end}}

			location / {
				set $backend_upstream "http://{{.Endpoint}}.default.svc.cluster.local";
				proxy_pass $backend_upstream;

				proxy_redirect off;
				proxy_set_header Host $host;
				proxy_connect_timeout 10s;
			}
		}
	{{end}}
`)

type configData strucct {
	DNSIP string
	Services string
}

func GenerateConfig(cnf *config.Config) (string, error) {
	ips, err := net.LookupIP("kube-dns.kube-system.svc.cluster.local")
	if err != nil {
	  return "", errors.Trace(err)
	}
	if len(ips) == 0 {
		return "", errors.New("cannot found any IP for the kube-dns internal endpoint")
	}

	services := bytes.NewBuffer(nil)
	nginx, err := servicesTemplate.Execute(services, cnf.Services)
	if err != nil {
	  return "", errors.Trace(err)
	}

	data := &configData{
		DNSIP: string(ips[0]),
		Services: services.String(),
	}
	buf := bytes.NewBuffer(nil)
	nginx, err := configTemplate.Execute(buf, data)
	if err != nil {
	  return "", errors.Trace(err)
	}

	return buf.String(), nil
}
