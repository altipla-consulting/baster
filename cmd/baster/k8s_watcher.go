package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
)

type KubernetesWatcher struct{}

func NewKubernetesWatcher() *KubernetesWatcher {
	return new(KubernetesWatcher)
}

func (watcher *KubernetesWatcher) Run() {
	go func() {
		if err := watcher.entrypoint(); err != nil {
			log.Fatal(errors.ErrorStack(err))
		}
	}()
}

func (watcher *KubernetesWatcher) entrypoint() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return errors.Trace(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return errors.Trace(err)
	}

	for {
		services, err := clientset.CoreV1().Services("default").List(metav1.ListOptions{})
		if err != nil {
			log.WithFields(log.Fields{"error": errors.ErrorStack(err)}).Warning("cannot list services")
			time.Sleep(10 * time.Second)
			continue
		}

		log.Println(services)
		time.Sleep(5 * time.Minute)
	}

	return nil
}
