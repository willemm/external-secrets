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
	// "encoding/json"
	"fmt"
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1alpha1"
	// smmeta "github.com/external-secrets/external-secrets/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"github.com/external-secrets/external-secrets/pkg/provider"
	"github.com/external-secrets/external-secrets/pkg/provider/schema"
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

func init() {
	schema.Register(&Provider{}, &esv1.SecretStoreProvider{
		Webhook: &esv1.WebhookProvider{},
	})
}

func (p *Provider) NewClient(ctx context.Context, store esv1.GenericStore, kube client.Client, namespace string) provider.SecretsClient {
	whClient := &WebHook{
		kube:      kube,
		store:     store,
		namespace: namespace,
	}
	return whClient, nil
}

func (w *WebHook) GetSecret(ctx context.Context, ref esv1.ExternalSecretDataRemoteRef) ([]byte, error) {
	fmt.PrintF("TODO: Get secret %v (from %v)\n", ref.Key, w.store)
}
