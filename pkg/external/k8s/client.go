package k8s

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/juju/errors"
)

func PodServiceAccount() (string, error) {
	token, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return "", errors.Trace(err)
	}

	return string(token), nil
}

type Client struct {
	token string
}

func NewClient(token string) *Client {
	return &Client{token}
}

func NewPodClient() (*Client, error) {
	token, err := PodServiceAccount()
	if err != nil {
		return nil, errors.Trace(err)
	}

	return NewClient(token), nil
}

func (client *Client) GetConfigMap(name string) (*ConfigMap, error) {
	cm := new(ConfigMap)
	if err := client.callReply("GET", fmt.Sprintf("/api/v1/namespaces/default/configmaps/%s", name), false, nil, cm); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}

		return nil, errors.Trace(err)
	}

	return cm, nil
}

func (client *Client) callReply(method, path string, hasRequest bool, request, reply interface{}) error {
	var body []byte
	if hasRequest {
		var err error
		body, err = json.Marshal(request)
		if err != nil {
			return errors.Trace(err)
		}
	}

	resp, err := client.call(method, path, body)
	if err != nil {
		return errors.Trace(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return errors.NotFoundf("resource")
	}

	if err := json.NewDecoder(resp.Body).Decode(reply); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (client *Client) call(method, path string, body []byte) (*http.Response, error) {
	req, _ := http.NewRequest(method, fmt.Sprintf("https://kubernetes%s", path), bytes.NewBuffer(body))
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	hc := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	return hc.Do(req)
}
