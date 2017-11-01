package main

import (
	"context"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"
)

const (
	KindLock        = "Lock"
	KindCertificate = "Certificate"
)

type Lock struct {
	// Key
	Hostname string `datastore:",noindex"`

	Acquired time.Time
}

func (lock *Lock) Key() *datastore.Key {
	return &datastore.Key{
		Kind: KindLock,
		Name: lock.Hostname,
	}
}

func acquireLock(ctx context.Context, client *datastore.Client, hostname string) error {
	for {
		lock := &Lock{Hostname: hostname}
		if err := client.Get(ctx, lock.Key(), lock); err != nil {
			if err == datastore.ErrNoSuchEntity {
				lock.Acquired = time.Now()
				if _, err := client.Put(ctx, lock.Key(), lock); err != nil {
					return errors.Trace(err)
				}

				log.WithFields(log.Fields{"hostname": hostname}).Info("lock acquired")

				return nil
			}

			return errors.Trace(err)
		}

		if lock.Acquired.Before(time.Now().Add(-24 * time.Hour)) {
			log.WithFields(log.Fields{"hostname": hostname}).Error("cleaning old stuck lock")
			if err := client.Delete(ctx, lock.Key()); err != nil {
				return errors.Trace(err)
			}

			continue
		}

		log.WithFields(log.Fields{"hostname": hostname}).Warning("log already acquired, waiting 30 seconds to try again")
		time.Sleep(30 * time.Second)
	}

	return nil
}

func freeLock(ctx context.Context, client *datastore.Client, hostname string) error {
	lock := &Lock{Hostname: hostname}
	if err := client.Delete(ctx, lock.Key()); err != nil {
		return errors.Trace(err)
	}

	log.WithFields(log.Fields{"hostname": hostname}).Info("free lock")
	return nil
}

type Cache struct {
	// Key
	Name string `datastore:",noindex"`

	Value []byte
}

func (cache *Cache) Key() *datastore.Key {
	return &datastore.Key{
		Kind: KindLock,
		Name: cache.Name,
	}
}

func getCache(ctx context.Context, client *datastore.Client, name string) ([]byte, error) {
	cache := &Cache{Name: name}
	if err := client.Get(ctx, cache.Key(), cache); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, nil
		}

		return nil, errors.Trace(err)
	}

	return cache.Value, nil
}

func setCache(ctx context.Context, client *datastore.Client, name string, value []byte) error {
	cache := &Cache{
		Name:  name,
		Value: value,
	}
	if _, err := client.Put(ctx, cache.Key(), cache); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func deleteCache(ctx context.Context, client *datastore.Client, name string) error {
	cache := &Cache{Name: name}
	return errors.Trace(client.Delete(ctx, cache.Key()))
}
