package k8s

type ConfigMap struct {
	Metadata *Metadata         `json:"metadata"`
	Data     map[string]string `json:"data"`
}

type Metadata struct {
	ResourceVersion string `json:"resourceVersion"`
}
