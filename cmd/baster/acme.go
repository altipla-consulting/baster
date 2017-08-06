package main

import (
	"context"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/datastore"
	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

const (
	KindCache = "Cache"
	Namespace = "baster"
)

type CacheModel struct {
	Data []byte
}

func (model *CacheModel) Key(key string) *datastore.Key {
	return &datastore.Key{
		Kind:      KindCache,
		Name:      key,
		Namespace: Namespace,
	}
}

type DatastoreCache struct {
	client *datastore.Client
}

func NewDatastoreCache(cnf *Config) (*DatastoreCache, error) {
	project := "test-project"
	if !IsDebug() {
		var err error
		project, err = metadata.ProjectID()
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	config, err := google.JWTConfigFromJSON([]byte(cnf.GoogleServiceAccount), datastore.ScopeDatastore)
	if err != nil {
		return nil, errors.Trace(err)
	}
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, project, option.WithTokenSource(config.TokenSource(ctx)))
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &DatastoreCache{client}, nil
}

func (cache *DatastoreCache) Get(ctx context.Context, key string) ([]byte, error) {
	log.WithFields(log.Fields{"key": key}).Info("get autocert cache key")

	model := new(CacheModel)
	if err := cache.client.Get(ctx, model.Key(key), model); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, autocert.ErrCacheMiss
		}

		return nil, errors.Trace(err)
	}

	return model.Data, autocert.ErrCacheMiss
}

func (cache *DatastoreCache) Put(ctx context.Context, key string, data []byte) error {
	log.WithFields(log.Fields{"key": key}).Info("put autocert cache key")

	model := &CacheModel{
		Data: data,
	}
	if _, err := cache.client.Put(ctx, model.Key(key), model); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (cache *DatastoreCache) Delete(ctx context.Context, key string) error {
	log.WithFields(log.Fields{"key": key}).Info("delete autocert cache key")

	model := new(CacheModel)
	if err := cache.client.Delete(ctx, model.Key(key)); err != nil {
		return errors.Trace(err)
	}

	return nil
}
