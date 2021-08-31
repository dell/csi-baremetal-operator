package csiwebhook

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	certificates "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	svcName   = "csi-webhook"
	namespace = "default"

	csrName    = svcName + "-" + "-csr"
	secretName = svcName + "-" + "-secret"
	cfgName    = svcName + "-" + "-cfg"

	dns1 = svcName
	dns2 = dns1 + "." + namespace
	dns3 = dns2 + "." + "svc"
)

type WebhookServer struct {
	Clientset kubernetes.Clientset
	logr.Logger
}

func (ws *WebhookServer) Generate(ctx context.Context) error {
	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	csrReq := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   svcName,
			Organization: []string{"dell.com"},
		},
		SignatureAlgorithm: x509.SHA512WithRSA,
		DNSNames:           []string{dns1, dns2, dns3},
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &csrReq, privKey)
	if err != nil {
		return err
	}

	csrPEM := new(bytes.Buffer)
	_ = pem.Encode(csrPEM, &pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrBytes,
	})
	if err != nil {
		return err
	}

	csr := &certificates.CertificateSigningRequest{
		ObjectMeta: v1.ObjectMeta{
			Name: csrName,
		},
		Spec: certificates.CertificateSigningRequestSpec{
			Groups: []string{
				"system:authenticated",
			},
			Usages: []certificates.KeyUsage{
				certificates.UsageDigitalSignature,
				certificates.UsageKeyEncipherment,
				certificates.UsageServerAuth,
			},
			Request: csrPEM.Bytes(),
		},
	}

	csrClient := ws.Clientset.CertificatesV1beta1().CertificateSigningRequests()
	_, err = csrClient.Update(ctx, csr, v1.UpdateOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if k8serrors.IsNotFound(err) {
		_, err = csrClient.Create(ctx, csr, v1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	csr.Status.Conditions = append(csr.Status.Conditions, certificates.CertificateSigningRequestCondition{
		Type:           certificates.CertificateApproved,
		Message:        "This CSR was approved by csi-baremetal-operator",
		LastUpdateTime: v1.Now(),
	})

	_, err = csrClient.UpdateApproval(ctx, csr, v1.UpdateOptions{})
	if err != nil {
		return err
	}

	for i := 0; i < 10; i++ {
		csr, err := csrClient.Get(ctx, csrName, v1.GetOptions{})
		if err != nil {
			return err
		}

		if csr.Status.Certificate != nil {
			break
		}

		time.Sleep(1 * time.Second)
	}

	keyPEM := new(bytes.Buffer)
	_ = pem.Encode(keyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})

	cert := csr.Status.Certificate

	tlsSecret := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name: secretName,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.key": keyPEM.Bytes(),
			"tls.crt": cert,
		},
	}

	secretClient := ws.Clientset.CoreV1().Secrets(namespace)
	_, err = secretClient.Update(ctx, tlsSecret, v1.UpdateOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if k8serrors.IsNotFound(err) {
		_, err = secretClient.Create(ctx, tlsSecret, v1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	path := "/validate"
	fail := admissionregistrationv1.Fail
	scope := admissionregistrationv1.NamespacedScope
	side := admissionregistrationv1.SideEffectClassNone

	mutateconfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: cfgName,
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{{
			Name: "mapplication.kb.io",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: cert,
				Service: &admissionregistrationv1.ServiceReference{
					Name:      svcName,
					Namespace: namespace,
					Path:      &path,
				},
			},
			Rules: []admissionregistrationv1.RuleWithOperations{{
				Operations: []admissionregistrationv1.OperationType{
					admissionregistrationv1.Create,
				},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"csi-baremetal.dell.com"},
					APIVersions: []string{"v1"},
					Resources:   []string{"deployments"},
					Scope:       &scope,
				},
			}},
			SideEffects:             &side,
			FailurePolicy:           &fail,
			AdmissionReviewVersions: []string{"v1", "v1beta"},
		}},
	}

	cfgClient := ws.Clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations()
	_, err = cfgClient.Update(ctx, mutateconfig, v1.UpdateOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if k8serrors.IsNotFound(err) {
		_, err = cfgClient.Create(ctx, mutateconfig, v1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	pair, err := tls.X509KeyPair(cert, keyPEM.Bytes())
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle(path, &WebhookHandler{ws.Logger})
	server := &http.Server{
		Addr:    ":8443",
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{pair},
		},
	}

	go func() {
		err := server.ListenAndServeTLS("", "")
		if err != nil {
			ws.Logger.Error(err, "webhook server failed")
			os.Exit(1)
		}
	}()

	return nil
}

type WebhookHandler struct {
	logr.Logger
}

func (wh *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wh.Logger.Info(fmt.Sprintf("%+v", w))
	wh.Logger.Info(fmt.Sprintf("%+v", r))
}
