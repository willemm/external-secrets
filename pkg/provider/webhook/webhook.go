/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	tpl "text/template"

	"github.com/Masterminds/sprig"
	"github.com/PaesslerAG/jsonpath"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	esv1alpha1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1alpha1"
	esmeta "github.com/external-secrets/external-secrets/apis/meta/v1"
	"github.com/external-secrets/external-secrets/pkg/provider"
	"github.com/external-secrets/external-secrets/pkg/provider/schema"
	"github.com/external-secrets/external-secrets/pkg/template"
)

// Provider satisfies the provider interface.
type Provider struct{}

type WebHook struct {
	kube      client.Client
	store     esv1alpha1.GenericStore
	namespace string
	storeKind string
}

var (
	log = ctrl.Log.WithName("webhook")
)

func init() {
	schema.Register(&Provider{}, &esv1alpha1.SecretStoreProvider{
		Webhook: &esv1alpha1.WebhookProvider{},
	})
}

func (p *Provider) NewClient(ctx context.Context, store esv1alpha1.GenericStore, kube client.Client, namespace string) (provider.SecretsClient, error) {
	whClient := &WebHook{
		kube:      kube,
		store:     store,
		namespace: namespace,
		storeKind: store.GetObjectKind().GroupVersionKind().Kind,
	}
	return whClient, nil
}

func getProvider(store esv1alpha1.GenericStore) (*esv1alpha1.WebhookProvider, error) {
	spc := store.GetSpec()
	if spc == nil || spc.Provider == nil || spc.Provider.Webhook == nil {
		return nil, fmt.Errorf("Missing store provider webhook")
	}
	return spc.Provider.Webhook, nil
}

func (w *WebHook) getStoreSecret(ctx context.Context, ref esmeta.SecretKeySelector) (*corev1.Secret, error) {
	ke := client.ObjectKey{
		Name:      ref.Name,
		Namespace: w.namespace,
	}
	if w.storeKind == esv1alpha1.ClusterSecretStoreKind {
		if ref.Namespace == nil {
			return nil, fmt.Errorf("no namespace on ClusterSecretStore webhook secret %s", ref.Name)
		}
		ke.Namespace = *ref.Namespace
	}
	secret := &corev1.Secret{}
	if err := w.kube.Get(ctx, ke, secret); err != nil {
		return nil, fmt.Errorf("Failed to get clustersecretstore webhook secret %s: %w", ref.Name, err)
	}
	return secret, nil
}

func (w *WebHook) GetSecret(ctx context.Context, ref esv1alpha1.ExternalSecretDataRemoteRef) ([]byte, error) {
	prov, err := getProvider(w.store)
	if err != nil {
		return nil, fmt.Errorf("Failed to get store: %w", err)
	}
	data := map[string]map[string]string{
		"remoteRef": {
			"key": url.QueryEscape(ref.Key),
		},
	}
	if prov.Secrets != nil {
		for _, secref := range prov.Secrets {
			if _, ok := data[secref.Name]; !ok {
				data[secref.Name] = make(map[string]string)
			}
			secret, err := w.getStoreSecret(ctx, secref.SecretRef)
			if err != nil {
				return nil, err
			}
			for sKey, sVal := range secret.Data {
				data[secref.Name][sKey] = string(sVal)
			}
		}
	}
	method := prov.Method
	if method == "" {
		method = "GET"
	}
	url, err := executeTemplateString(prov.Url, data)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse url: %w", err)
	}
	body, err := executeTemplate(prov.Body, data)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, &body)
	if err != nil {
		return nil, fmt.Errorf("Failed to call endpoint: %w", err)
	}
	if prov.Headers != nil {
		for hKey, hValueTpl := range prov.Headers {
			hValue, err := executeTemplateString(hValueTpl, data)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse header %s: %w", hKey, err)
			}
			req.Header.Add(hKey, hValue)
		}
	}

	client, err := w.getHttpClient(ctx, prov)
	if err != nil {
		return nil, fmt.Errorf("Failed to call endpoint: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to call endpoint: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Endpoint gave error %s", resp.Status)
	}
	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read response: %w", err)
	}
	if prov.Result.JsonPath != "" {
		jsondata := interface{}(nil)
		if err := json.Unmarshal(result, &jsondata); err != nil {
			return nil, fmt.Errorf("Failed to parse response: %w", err)
		}
		jsondata, err = jsonpath.Get(prov.Result.JsonPath, jsondata)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse response (path): %w", err)
		}
		jsonvalue, ok := jsondata.(string)
		if !ok {
			return nil, fmt.Errorf("Failed to parse response (wrong type)")
		}
		return []byte(jsonvalue), nil
	}

	return result, nil
}

func (w *WebHook) GetSecretMap(ctx context.Context, ref esv1alpha1.ExternalSecretDataRemoteRef) (map[string][]byte, error) {
	return nil, fmt.Errorf("GetSecretMap not implemented")
}

func (w *WebHook) getHttpClient(ctx context.Context, provider *esv1alpha1.WebhookProvider) (*http.Client, error) {
	client := &http.Client{}
	if len(provider.CABundle) == 0 && provider.CAProvider == nil {
		return client, nil
	}
	caCertPool := x509.NewCertPool()
	if len(provider.CABundle) > 0 {
		ok := caCertPool.AppendCertsFromPEM(provider.CABundle)
		if !ok {
			return nil, fmt.Errorf("Failed to append cabundle")
		}
	}

	if provider.CAProvider != nil && w.storeKind == esv1alpha1.ClusterSecretStoreKind && provider.CAProvider.Namespace == nil {
		return nil, fmt.Errorf("Missing namespace on CAProvider secret")
	}

	if provider.CAProvider != nil {
		var cert []byte
		var err error

		switch provider.CAProvider.Type {
		case esv1alpha1.WebhookCAProviderTypeSecret:
			cert, err = w.getCertFromSecret(provider)
		case esv1alpha1.WebhookCAProviderTypeConfigMap:
			cert, err = w.getCertFromConfigMap(provider)
		default:
			return nil, fmt.Errorf("Unknown caprovider type: %s", provider.CAProvider.Type)
		}

		if err != nil {
			return nil, err
		}

		ok := caCertPool.AppendCertsFromPEM(cert)
		if !ok {
			return nil, fmt.Errorf("Failed to append cabundle")
		}
	}

	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.TLSClientConfig.RootCAs = caCertPool
	}
	return client, nil
}

func (w *WebHook) getCertFromSecret(provider *esv1alpha1.WebhookProvider) ([]byte, error) {
	secretRef := esmeta.SecretKeySelector{
		Name: provider.CAProvider.Name,
		Key:  provider.CAProvider.Key,
	}

	if provider.CAProvider.Namespace != nil {
		secretRef.Namespace = provider.CAProvider.Namespace
	}

	ctx := context.Background()
	res, err := w.secretKeyRef(ctx, &secretRef)
	if err != nil {
		return nil, err
	}

	return []byte(res), nil
}

func (w *WebHook) secretKeyRef(ctx context.Context, secretRef *esmeta.SecretKeySelector) (string, error) {
	secret := &corev1.Secret{}
	ref := client.ObjectKey{
		Namespace: w.namespace,
		Name:      secretRef.Name,
	}
	if (w.storeKind == esv1alpha1.ClusterSecretStoreKind) &&
		(secretRef.Namespace != nil) {
		ref.Namespace = *secretRef.Namespace
	}
	err := w.kube.Get(ctx, ref, secret)
	if err != nil {
		return "", err
	}

	keyBytes, ok := secret.Data[secretRef.Key]
	if !ok {
		return "", err
	}

	value := string(keyBytes)
	valueStr := strings.TrimSpace(value)
	return valueStr, nil
}

func (w *WebHook) getCertFromConfigMap(provider *esv1alpha1.WebhookProvider) ([]byte, error) {
	objKey := client.ObjectKey{
		Name: provider.CAProvider.Name,
	}

	if provider.CAProvider.Namespace != nil {
		objKey.Namespace = *provider.CAProvider.Namespace
	}

	configMapRef := &corev1.ConfigMap{}
	ctx := context.Background()
	err := w.kube.Get(ctx, objKey, configMapRef)
	if err != nil {
		return nil, fmt.Errorf("Failed to get caprovider secret %s: %w", objKey.Name, err)
	}

	val, ok := configMapRef.Data[provider.CAProvider.Key]
	if !ok {
		return nil, fmt.Errorf("Failed to get caprovider configmap %s: %w", objKey.Name, provider.CAProvider.Key)
	}

	return []byte(val), nil
}

func (w *WebHook) Close(ctx context.Context) error {
	return nil
}

func executeTemplateString(tmpl string, data map[string]map[string]string) (string, error) {
	result, err := executeTemplate(tmpl, data)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

func executeTemplate(tmpl string, data map[string]map[string]string) (bytes.Buffer, error) {
	var result bytes.Buffer
	if tmpl == "" {
		return result, nil
	}
	urlt, err := tpl.New("webhooktemplate").Funcs(sprig.TxtFuncMap()).Funcs(template.FuncMap()).Parse(tmpl)
	if err != nil {
		return result, err
	}
	if err := urlt.Execute(&result, data); err != nil {
		return result, err
	}
	return result, nil
}
