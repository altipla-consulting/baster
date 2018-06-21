package kubernetes

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/juju/errors"
)

type Client struct {
	token     string
	hc        *http.Client
	namespace string
}

func NewClient() (*Client, error) {
	token, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, errors.Trace(err)
	}

	certs, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		return nil, errors.Trace(err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(certs) {
		return nil, errors.NotValidf("certificate")
	}

	hc := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: pool,
			},
		},
	}

	namespace, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &Client{
		token:     string(token),
		hc:        hc,
		namespace: string(namespace),
	}, nil
}

func (client *Client) callGet(endpoint string, result interface{}) error {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://kubernetes.default.svc%s", endpoint), nil)
	if err != nil {
		return errors.Trace(err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.token))

	resp, err := client.hc.Do(req)
	if err != nil {
		return errors.Trace(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("unexpected API status: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return errors.Trace(err)
	}

	return nil
}

type ConfigMap struct {
	Data     map[string]string `json:"data"`
	Metadata *ObjectMeta       `json:"metadata"`
}

type ObjectMeta struct {
	ResourceVersion string `json:"resourceVersion"`
}

func (client *Client) GetConfigMap(name string) (*ConfigMap, error) {
	reply := new(ConfigMap)
	endpoint := fmt.Sprintf("/api/v1/namespaces/%s/configmaps/%s", client.namespace, name)
	if err := client.callGet(endpoint, reply); err != nil {
		return nil, errors.Trace(err)
	}

	return reply, nil
}
