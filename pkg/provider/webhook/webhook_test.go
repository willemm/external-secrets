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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	esv1alpha1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1alpha1"
)

type testCase struct {
	Case string `json:"case,omitempty"`
	Args args   `json:"args"`
	Want want   `json:"want"`
}

type args struct {
	URL        string `json:"url,omitempty"`
	Key        string `json:"key,omitempty"`
	Body       string `json:"body,omitempty"`
	Version    string `json:"version,omitempty"`
	JSONPath   string `json:"jsonpath,omitempty"`
	Response   string `json:"response,omitempty"`
	StatusCode int    `json:"statuscode,omitempty"`
}

type want struct {
	Path   string `json:"path,omitempty"`
	Err    string `json:"err,omitempty"`
	Result string `json:"result,omitempty"`
}

var testCases = `
case: error url
args:
  url: /api/getsecret?id={{ .unclosed.template
want:
  err: failed to parse url
---
case: error body
args:
  url: /api/getsecret?id={{ .remoteRef.key }}&version={{ .remoteRef.version }}
  body: Body error {{ .unclosed.template
want:
  err: failed to parse body
---
case: error connection
args:
  url: 1/api/getsecret?id={{ .remoteRef.key }}&version={{ .remoteRef.version }}
want:
  err: failed to call endpoint
---
case: error not found
args:
  url: /api/getsecret?id={{ .remoteRef.key }}&version={{ .remoteRef.version }}
  key: testkey
  version: 1
  statuscode: 404
  response: not found
want:
  path: /api/getsecret?id=testkey&version=1
  err: endpoint gave error 404
---
case: error bad json
args:
  url: /api/getsecret?id={{ .remoteRef.key }}&version={{ .remoteRef.version }}
  key: testkey
  version: 1
  jsonpath: $.result.thesecret
  response: '{"result":{"thesecret":"secret-value"}'
want:
  path: /api/getsecret?id=testkey&version=1
  err: failed to parse response json
---
case: error bad jsonpath
args:
  url: /api/getsecret?id={{ .remoteRef.key }}&version={{ .remoteRef.version }}
  key: testkey
  version: 1
  jsonpath: $.result.thesecret
  response: '{"result":{"nosecret":"secret-value"}}'
want:
  path: /api/getsecret?id=testkey&version=1
  err: failed to get response path
---
case: error bad json data
args:
  url: /api/getsecret?id={{ .remoteRef.key }}&version={{ .remoteRef.version }}
  key: testkey
  version: 1
  jsonpath: $.result.thesecret
  response: '{"result":{"thesecret":{"one":"secret-value"}}}'
want:
  path: /api/getsecret?id=testkey&version=1
  err: failed to get response (wrong type)
---
case: good plaintext
args:
  url: /api/getsecret?id={{ .remoteRef.key }}&version={{ .remoteRef.version }}
  key: testkey
  version: 1
  response: secret-value
want:
  path: /api/getsecret?id=testkey&version=1
  err: ""
  result: secret-value
---
case: good json
args:
  url: /api/getsecret?id={{ .remoteRef.key }}&version={{ .remoteRef.version }}
  key: testkey
  version: 1
  jsonpath: $.result.thesecret
  response: '{"result":{"thesecret":"secret-value"}}'
want:
  path: /api/getsecret?id=testkey&version=1
  err: ""
  result: secret-value
`

func TestWebhookGetSecret(t *testing.T) {
	ydec := yaml.NewDecoder(bytes.NewReader([]byte(testCases)))
	for {
		var tc testCase
		if err := ydec.Decode(&tc); err != nil {
			if !errors.Is(err, io.EOF) {
				t.Errorf("testcase decode error %w", err)
			}
			break
		}
		runTestCase(tc, t)
	}
}

func runTestCase(tc testCase, t *testing.T) {
	// Start a new server for every test case because the server wants to check the expected api path
	ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if tc.Want.Path != "" && req.URL.String() != tc.Want.Path {
			t.Errorf("%s: unexpected api path: %s, expected %s", tc.Case, req.URL.String(), tc.Want.Path)
		}
		if tc.Args.StatusCode != 0 {
			rw.WriteHeader(tc.Args.StatusCode)
		}
		rw.Write([]byte(tc.Args.Response))
	}))
	defer ts.Close()

	testStore := makeClusterSecretStore(ts.URL, tc.Args)
	testProv := &Provider{}
	client, err := testProv.NewClient(context.Background(), testStore, nil, "testnamespace")
	if err != nil {
		t.Errorf("%s: error creating client: %s", tc.Case, err.Error())
		return
	}

	testRef := esv1alpha1.ExternalSecretDataRemoteRef{
		Key:     tc.Args.Key,
		Version: tc.Args.Version,
	}
	secret, err := client.GetSecret(context.Background(), testRef)
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	if !strings.Contains(errStr, tc.Want.Err) {
		t.Errorf("%s: unexpected error: '%s' (expected '%s')", tc.Case, errStr, tc.Want.Err)
	}
	if err == nil && string(secret) != tc.Want.Result {
		t.Errorf("%s: unexpected response: '%s' (expected '%s')", tc.Case, secret, tc.Want.Result)
	}
}

func makeClusterSecretStore(url string, args args) *esv1alpha1.ClusterSecretStore {
	return &esv1alpha1.ClusterSecretStore{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterSecretStore",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wehbook-store",
			Namespace: "default",
		},
		Spec: esv1alpha1.SecretStoreSpec{
			Provider: &esv1alpha1.SecretStoreProvider{
				Webhook: &esv1alpha1.WebhookProvider{
					URL:  url + args.URL,
					Body: args.Body,
					Headers: map[string]string{
						"Content-Type": "application.json",
						"X-SecretKey":  "{{ .remoteRef.key }}",
					},
					Result: esv1alpha1.WebhookResult{
						JSONPath: args.JSONPath,
					},
				},
			},
		},
	}
}
