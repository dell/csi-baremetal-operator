package csiwebhook

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"io/ioutil"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver"
	"k8s.io/apimachinery/pkg/runtime"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	Client    client.Client
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

	ws.Logger.Info("csr")

	csr.Status.Conditions = append(csr.Status.Conditions, certificates.CertificateSigningRequestCondition{
		Type:           certificates.CertificateApproved,
		Message:        "This CSR was approved by csi-baremetal-operator",
		LastUpdateTime: v1.Now(),
	})

	_, err = csrClient.UpdateApproval(ctx, csr, v1.UpdateOptions{})
	if err != nil {
		return err
	}

	gottenCert := false
	for i := 0; i < 20; i++ {
		csr, err = csrClient.Get(ctx, csrName, v1.GetOptions{})
		if err != nil {
			return err
		}

		if csr.Status.Certificate != nil {
			ws.Logger.Info(fmt.Sprintf("%+v", csr.Status.Certificate))
			gottenCert = true
			break
		}

		time.Sleep(1 * time.Second)
	}

	if !gottenCert {
		return errors.New("no csr certificate")
	}

	ws.Logger.Info("csr-approved")

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

	ws.Logger.Info("secret")

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
	_, err = cfgClient.Create(ctx, mutateconfig, v1.CreateOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if k8serrors.IsNotFound(err) {
		_, err = cfgClient.Create(ctx, mutateconfig, v1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	ws.Logger.Info("cfg")

	pair, err := tls.X509KeyPair(cert, keyPEM.Bytes())
	if err != nil {
		ws.Logger.Info("cert")
		ws.Logger.Info(string(cert))
		ws.Logger.Info("key")
		ws.Logger.Info(string(keyPEM.Bytes()))
		return err
	}

	mux := http.NewServeMux()
	mux.Handle(path, &WebhookHandler{
		Client: ws.Client,
		Logger: ws.Logger,
	})
	server := &http.Server{
		Addr:    ":8443",
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{pair},
		},
	}

	go func() {
		ws.Logger.Info("start server")
		err := server.ListenAndServeTLS("", "")
		if err != nil {
			ws.Logger.Error(err, "webhook server failed")
			os.Exit(1)
		}
	}()

	return nil
}

type WebhookHandler struct {
	Client client.Client
	logr.Logger
}

func (wh *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wh.Logger.Info("HANDLER")
	wh.Logger.Info(fmt.Sprintf("%+v", w))
	wh.Logger.Info(fmt.Sprintf("%+v", r))

	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	ar := admissionv1.AdmissionReview{}
	deserializer := apiserver.Codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		wh.Logger.Error(err, "")
	}

	wh.Logger.Info(fmt.Sprintf("%+v", ar))

	reviewResponse := &admissionv1.AdmissionResponse{}
	reviewResponse.Allowed = true

	ctx := context.Background()

	deployments := &csibaremetalv1.DeploymentList{}
	err := wh.Client.List(ctx, deployments)
	if err != nil {
		wh.Logger.Error(err, "")
	} else {
		if len(deployments.Items) > 0 {
			reviewResponse.Allowed = false
			reviewResponse.Result = &v1.Status{
				Reason: "deployment ... already exists",
			}
		}
	}

	response := ar
	if reviewResponse != nil {
		response.Response = reviewResponse
		response.Response.UID = ar.Request.UID
	}
	// reset the Object and OldObject, they are not needed in a response.
	ar.Request.Object = runtime.RawExtension{}
	ar.Request.OldObject = runtime.RawExtension{}

	resp, err := json.Marshal(response)
	if err != nil {
		wh.Logger.Error(err, "")
	}
	if _, err := w.Write(resp); err != nil {
		wh.Logger.Error(err, "")
	}
}
