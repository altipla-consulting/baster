package stores

import (
	"context"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/datastore"
	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/acme/autocert"
)

const (
	KindCache = "Cache"
	Namespace = "baster"
)

type CacheModel struct {
	Data []byte `datastore:",noindex"`
}

func (model *CacheModel) Key(key string) *datastore.Key {
	return &datastore.Key{
		Kind:      KindCache,
		Name:      key,
		Namespace: Namespace,
	}
}

type Datastore struct {
	client *datastore.Client
}

func NewDatastore() (*Datastore, error) {
	project, err := metadata.ProjectID()
	if err != nil {
		return nil, errors.Trace(err)
	}

	client, err := datastore.NewClient(context.Background(), project)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &Datastore{client}, nil
}

func (cache *Datastore) Get(ctx context.Context, key string) ([]byte, error) {
	log.WithFields(log.Fields{"key": key, "store": "datastore"}).Info("get key")

	model := new(CacheModel)
	if err := cache.client.Get(ctx, model.Key(key), model); err != nil {
		if err == datastore.ErrNoSuchEntity {
			log.Info("cache miss")
			return nil, autocert.ErrCacheMiss
		}

		log.WithFields(log.Fields{"error": err}).Error("cannot get key")
		return nil, errors.Trace(err)
	}

	return model.Data, nil
}

func (cache *Datastore) Put(ctx context.Context, key string, data []byte) error {
	log.WithFields(log.Fields{"key": key, "store": "datastore"}).Info("put key")

	model := &CacheModel{
		Data: data,
	}
	if _, err := cache.client.Put(ctx, model.Key(key), model); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("cannot put key")
		return errors.Trace(err)
	}

	return nil
}

func (cache *Datastore) Delete(ctx context.Context, key string) error {
	log.WithFields(log.Fields{"key": key, "store": "datastore"}).Info("delete key")

	model := new(CacheModel)
	if err := cache.client.Delete(ctx, model.Key(key)); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("cannot delete key")
		return errors.Trace(err)
	}

	return nil
}
