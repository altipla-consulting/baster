package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509/pkix"
	"crypto"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"net/http"
	"time"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/datastore"
	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/acme"

	"baster/pkg/config"
)

const (
	snakeoilKey  = "/etc/ssl/certs/ssl-cert-snakeoil.pem"
	snakeoilCert = "/etc/ssl/private/ssl-cert-snakeoil.key"
)

func verifyCertificates(cnf *config.Config) {
	ctx := context.Background()

	v, err := newVerifier(ctx, cnf)
	if err != nil {
		log.WithFields(log.Fields{"error": errors.ErrorStack(err)}).Error("cannot init verifier")
		return
	}

	for _, service := range cnf.Services {
		if err := v.serviceCert(ctx, service); err != nil {
			log.WithFields(log.Fields{"error": errors.ErrorStack(err), "service": service.Name}).Error("cannot verify service certificate")
		}
	}
}

type verifier struct {
	dsclient   *datastore.Client
	acmeClient *acme.Client
}

func newVerifier(ctx context.Context, cnf *config.Config) (*verifier, error) {
	v := new(verifier)

	project, err := metadata.ProjectID()
	if err != nil {
		return nil, errors.Trace(err)
	}
	v.dsclient, err = datastore.NewClient(context.Background(), project)
	if err != nil {
		return nil, errors.Trace(err)
	}

	v.acmeClient = &acme.Client{
		DirectoryURL: acme.LetsEncryptURL,
	}

	data, err := getCache(ctx, v.dsclient, "acme_account.key")
	if err != nil {
		return nil, errors.Trace(err)
	}
	if data == nil {
		log.Info("no acme account found, creating a new key")

		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, errors.Trace(err)
		}

		buf := bytes.NewBuffer(nil)
		if err := encodeECDSAKey(buf, key); err != nil {
			return nil, errors.Trace(err)
		}
		if err := setCache(ctx, v.dsclient, "acme_account.key", buf.Bytes()); err != nil {
			return nil, errors.Trace(err)
		}

		v.acmeClient.Key = key
	} else {
		log.Info("previous acme account found, restoring it")

		priv, _ := pem.Decode(data)
		if priv == nil || !strings.Contains(priv.Type, "PRIVATE") {
			return nil, errors.Errorf("invalid account key found in cache")
		}

		key, err := x509.ParseECPrivateKey(priv.Bytes)
		if err != nil {
			return nil, errors.Trace(err)
		}

		v.acmeClient.Key = key
	}

	account := &acme.Account{
		Contact: []string{cnf.ACME.Email},
	}
	if _, err := v.acmeClient.Register(ctx, account, func(string) bool { return true }); err != nil {
		if aerr, ok := err.(*acme.Error); !ok || aerr.StatusCode != http.StatusConflict {
			return nil, errors.Trace(err)
		}
	}

	return v, nil
}

func (v *verifier) serviceCert(ctx context.Context, service *config.Service) error {
	keyFile := fmt.Sprintf("/etc/certificates/%s.key", service.Name)
	certFile := fmt.Sprintf("/etc/certificates/%s.crt", service.Name)

	if service.Snakeoil {
		log.WithFields(log.Fields{"hostname": service.Hostname}).Info("preparing snakeoil certificates")

		if err := os.Link(keyFile, snakeoilKey); err != nil {
			return errors.Trace(err)
		}
		if err := os.Link(certFile, snakeoilCert); err != nil {
			return errors.Trace(err)
		}

		return nil
	}

	log.WithFields(log.Fields{"hostname": service.Hostname}).Info("preparing real certificates")
	if err := os.Remove(keyFile); err != nil && !os.IsNotExist(err) {
		return errors.Trace(err)
	}
	if err := os.Remove(certFile); err != nil && !os.IsNotExist(err) {
		return errors.Trace(err)
	}

	cert, err := v.getCert(ctx, service.Hostname)
	if err != nil {
		return errors.Trace(err)
	}
	for cert == nil {
		if err := v.createCert(ctx, service.Hostname); err != nil {
			return errors.Trace(err)
		}

		cert, err = v.getCert(ctx, service.Hostname)
		if err != nil {
			return errors.Trace(err)
		}

		if cert == nil {
			log.Error("something wrong is happening, we cannot get the created certificate, retrying in 30 seconds")
			time.Sleep(30 * time.Second)
			continue
		}

		break
	}

	data, err := x509.MarshalECPrivateKey(cert.PrivateKey.(*ecdsa.PrivateKey))
	if err != nil {
	  return errors.Trace(err)
	}
	privData := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: data,
	})
	if err := ioutil.WriteFile(keyFile, privData, 0600); err != nil {
		return errors.Trace(err)
	}

	var pubData []byte
	for _, c := range cert.Certificate {
		pubData = append(pubData, pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: c,
		})...)
	}
	if err := ioutil.WriteFile(certFile, pubData, 0600); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (v *verifier) createCert(ctx context.Context, hostname string) error {
	if err := acquireLock(ctx, v.dsclient, hostname); err != nil {
		return errors.Trace(err)
	}

	authz, err := v.acmeClient.Authorize(ctx, hostname)
	if err != nil {
		return errors.Trace(err)
	}
	if authz.Status == acme.StatusValid {
		log.Info("authentication is already valid, exit now the creation")
		return nil
	}

	log.WithFields(log.Fields{"hostname": hostname}).Info("creating new certificate")

	var challenge *acme.Challenge
	for _, c := range authz.Challenges {
		if c.Type == "http-01" {
			challenge = c
			break
		}
	}
	if challenge == nil {
		return errors.Errorf("no supported challenge type found: %+v", authz.Challenges)
	}

	value, err := v.acmeClient.HTTP01ChallengeResponse(challenge.Token)
	if err != nil {
		return errors.Trace(err)
	}
	if err := setCache(ctx, v.dsclient, fmt.Sprintf("token.%s", challenge.Token), []byte(value)); err != nil {
		return errors.Trace(err)
	}

	if _, err := v.acmeClient.Accept(ctx, challenge); err != nil {
		return errors.Trace(err)
	}
	log.WithFields(log.Fields{"hostname": hostname}).Info("waiting authorization...")
	if _, err := v.acmeClient.WaitAuthorization(ctx, authz.URI); err != nil {
		return errors.Trace(err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return errors.Trace(err)
	}

	req := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: hostname},
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, req, key)
	if err != nil {
		return errors.Trace(err)
	}
	der, _, err := v.acmeClient.CreateCert(ctx, csr, 0, true)
	if err != nil {
		return errors.Trace(err)
	}
	leaf, err := validCert(hostname, der, key)
	if err != nil {
		return errors.Trace(err)
	}

	if err := freeLock(ctx, v.dsclient, hostname); err != nil {
		return errors.Trace(err)
	}

	cert := &tls.Certificate{
		PrivateKey:  key,
		Certificate: der,
		Leaf:        leaf,
	}
	if err := v.storeCert(ctx, hostname, cert); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (v *verifier) storeCert(ctx context.Context, hostname string, cert *tls.Certificate) error {
	buf := bytes.NewBuffer(nil)

	// Private
	key := cert.PrivateKey.(*ecdsa.PrivateKey)
	if err := encodeECDSAKey(buf, key); err != nil {
		return errors.Trace(err)
	}

	// Public
	for _, b := range cert.Certificate {
		pb := &pem.Block{Type: "CERTIFICATE", Bytes: b}
		if err := pem.Encode(buf, pb); err != nil {
			return errors.Trace(err)
		}
	}

	if err := setCache(ctx, v.dsclient, hostname, buf.Bytes()); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (v *verifier) getCert(ctx context.Context, hostname string) (*tls.Certificate, error) {
	data, err := getCache(ctx, v.dsclient, hostname)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Private
	priv, pub := pem.Decode(data)
	if priv == nil || !strings.Contains(priv.Type, "PRIVATE") {
		return nil, nil
	}
	privKey, err := x509.ParseECPrivateKey(priv.Bytes)
	if err != nil {
		return nil, nil
	}

	// Public
	var pubDER [][]byte
	for len(pub) > 0 {
		var b *pem.Block
		b, pub = pem.Decode(pub)
		if b == nil {
			break
		}
		pubDER = append(pubDER, b.Bytes)
	}
	if len(pub) > 0 {
		// Leftover content not consumed by pem.Decode. Corrupt. Ignore.
		return nil, nil
	}

	leaf, err := validCert(hostname, pubDER, privKey)
	if err != nil {
		return nil, nil
	}

	return &tls.Certificate{
		Certificate: pubDER,
		PrivateKey:  privKey,
		Leaf:        leaf,
	}, nil
}

func encodeECDSAKey(w io.Writer, key *ecdsa.PrivateKey) error {
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return errors.Trace(err)
	}
	pb := &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	return errors.Trace(pem.Encode(w, pb))
}

func validCert(hostname string, der [][]byte, key crypto.Signer) (*x509.Certificate, error) {
	// parse public part(s)
	var n int
	for _, b := range der {
		n += len(b)
	}
	pub := make([]byte, n)
	n = 0
	for _, b := range der {
		n += copy(pub[n:], b)
	}
	x509Cert, err := x509.ParseCertificates(pub)
	if len(x509Cert) == 0 {
		return nil, errors.New("no public key found")
	}

	// verify the leaf is not expired and matches the hostname name
	leaf := x509Cert[0]
	now := timeNow()
	if now.Before(leaf.NotBefore) {
		return nil, errors.New("certificate is not valid yet")
	}
	if now.After(leaf.NotAfter) {
		return nil, errors.New("expired certificate")
	}
	if err := leaf.VerifyHostname(hostname); err != nil {
		return nil, err
	}

	// ensure the leaf corresponds to the private key
	switch pub := leaf.PublicKey.(type) {
	case *ecdsa.PublicKey:
		prv, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key type does not match public key type")
		}
		if pub.X.Cmp(prv.X) != 0 || pub.Y.Cmp(prv.Y) != 0 {
			return nil, errors.New("private key does not match public key")
		}

	default:
		return nil, errors.New("unknown public key algorithm")
	}

	return leaf, nil
}
