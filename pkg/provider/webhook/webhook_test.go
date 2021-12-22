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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	esv1alpha1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1alpha1"

	"fmt"
)

type testCase struct {
	Reason string `json:"reason,omitempty"`
	Args   args   `json:"args"`
	Want   want   `json:"want"`
}

type args struct {
	URL      string `json:"url"`
	Key      string `json:"key"`
	Version  string `json:"version,omitempty"`
	JSONPath string `json:"jsonpath,omitempty"`
	Response string `json:"response,omitempty"`
}

type want struct {
	Path   string `json:"path"`
	Err    string `json:"err,omitempty"`
	Result string `json:"result,omitempty"`
}

var testCases = map[string]string{
	"get secret simple": `
args:
  url: /api/getsecret?id={{ .remoteRef.key }}&version={{ .remoteRef.version }}
  key: testkey
  version: "1"
  jsonpath: $.result.thesecret
  response: '{"result":{"thesecret":"secret-value"}}'
want:
  path: /api/getsecret?id=testkey&version=1
  err: ""
  result: secret-value
`,
}

func TestWebhookGetSecret(t *testing.T) {
	for testname, testcaseyaml := range testCases {
		testcase := &testCase{}
		yaml.Unmarshal([]byte(testcaseyaml), testcase)
		if testcase.Reason == "" {
			testcase.Reason = testname
		}
		runTestCase(testcase, t)
	}
}

func runTestCase(tc *testCase, t *testing.T) {
	// Start a new server for every test case because the server wants to check the expected api path
	ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if tc.Want.Path != "" && req.URL.String() != tc.Want.Path {
			t.Errorf("%s: unexpected api path: %s, expected %s", tc.Reason, req.URL.String(), tc.Want.Path)
		}
		rw.Write([]byte(tc.Args.Response))
	}))
	defer ts.Close()

	testStore := makeClusterSecretStore(ts.URL+tc.Args.URL, tc.Args.JSONPath)
	testProv := &Provider{}
	client, err := testProv.NewClient(context.Background(), testStore, nil, "testnamespace")
	if err != nil {
		t.Errorf("%s: error creating client: %s", tc.Reason, err.Error())
		return
	}

	testRef := esv1alpha1.ExternalSecretDataRemoteRef{
		Key:     tc.Args.Key,
		Version: tc.Args.Version,
	}
	secret, err := client.GetSecret(context.Background(), testRef)
	if !errorContains(err, tc.Want.Err) {
		t.Errorf("%s: unexpected error: %s (expected '%s')", tc.Reason, err.Error(), tc.Want.Err)
	}
	if err == nil && string(secret) != tc.Want.Result {
		t.Errorf("%s:unexpected response: %s (expected '%s')", tc.Reason, secret, tc.Want.Result)
	}
}

func errorContains(out error, want string) bool {
	if out == nil {
		return want == ""
	}
	if want == "" {
		return false
	}
	return strings.Contains(out.Error(), want)
}

func makeClusterSecretStore(url, jsonpath string) *esv1alpha1.ClusterSecretStore {
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
					URL: url,
					Headers: map[string]string{
						"Content-Type": "application.json",
						"X-SecretKey":  "{{ .remoteRef.key }}",
					},
					Result: esv1alpha1.WebhookResult{
						JSONPath: jsonpath,
					},
				},
			},
		},
	}
}
