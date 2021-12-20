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
	"context"
	"encoding/json"
	"fmt"
	"io"

	"bytes"
	tpl "text/template"
	"github.com/external-secrets/external-secrets/pkg/template"
	"github.com/Masterminds/sprig"

	"net/url"
	"net/http"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1alpha1"
	// smmeta "github.com/external-secrets/external-secrets/apis/meta/v1"
	"github.com/external-secrets/external-secrets/pkg/provider"
	"github.com/external-secrets/external-secrets/pkg/provider/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/PaesslerAG/jsonpath"
	// corev1 "k8s.io/api/core/v1"
	// "strings"
)

// Provider satisfies the provider interface.
type Provider struct{}

type WebHook struct {
	kube      client.Client
	store     esv1.GenericStore
	namespace string
}

var (
	log = ctrl.Log.WithName("webhook")
)

func init() {
	schema.Register(&Provider{}, &esv1.SecretStoreProvider{
		Webhook: &esv1.WebhookProvider{},
	})
}

func (p *Provider) NewClient(ctx context.Context, store esv1.GenericStore, kube client.Client, namespace string) (provider.SecretsClient, error) {
	whClient := &WebHook{
		kube:      kube,
		store:     store,
		namespace: namespace,
	}
	return whClient, nil
}

func getProvider(store esv1.GenericStore) (*esv1.WebhookProvider, error) {
	spc := store.GetSpec()
	if spc == nil || spc.Provider == nil || spc.Provider.Webhook == nil {
		return nil, fmt.Errorf("Missing store provider webhook")
	}
	return spc.Provider.Webhook, nil
}

func (w *WebHook) GetSecret(ctx context.Context, ref esv1.ExternalSecretDataRemoteRef) ([]byte, error) {
	prov, err := getProvider(w.store)
	if err != nil {
		return nil, fmt.Errorf("Failed to get store: %w", err)
	}
	data := map[string]map[string]string {
		"remoteRef": map[string]string {
			"key": url.QueryEscape(ref.Key),
		},
	}
	// TODO: Extra secrets
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
		for hKey,hValueTpl := range prov.Headers {
			hValue, err := executeTemplateString(hValueTpl, data)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse header %s: %w", hKey, err)
			}
			log.Info("Adding header", "header", hKey, "value", hValue)
			req.Header.Add(hKey, hValue)
		}
	}

	client := &http.Client{}
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
		jsondata, err = jsonpath.Get(prov.Result.JsonPath, &jsondata)
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

func (w *WebHook) GetSecretMap(ctx context.Context, ref esv1.ExternalSecretDataRemoteRef) (map[string][]byte, error) {
	log.Info("TODO: Get secret map", "ref", ref.Key, "store", w.store)
	return nil, fmt.Errorf("GetSecretMap not implemented")
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
