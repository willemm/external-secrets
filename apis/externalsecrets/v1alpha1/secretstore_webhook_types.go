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

package v1alpha1

import (
	esmeta "github.com/external-secrets/external-secrets/apis/meta/v1"
)

// AkeylessProvider Configures an store to sync secrets using Akeyless KV.
type WebhookProvider struct {
	// Webhook Method
	// +optional, default GET
	Method string `json:"method,omitempty"`

	// Webhook url to call
	Url string `json:"url"`

	// Headers
	// +optional
	Headers map[string]string `json:"headers,omitempty"`

	// Body
	// +optional
	Body string `json:"body,omitempty"`

	// Result formatting
	Result WebhookResult `json:"result"`

	// Secrets to fill in templates
	// +optional
	Secrets []WebhookSecret `json:"secrets,omitempty"`
}

type WebhookResult struct {
	// Json path of return value
	// +optional
	JsonPath string `json:"jsonPath,omitempty"`
}

type WebhookSecret struct {
	// Name of this secret in templates
	Name string `json:"name"`

	// Secret ref to fill in credentials
	SecretRef esmeta.SecretKeySelector `json:"secretRef"`
}
