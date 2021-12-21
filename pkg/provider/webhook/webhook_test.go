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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	esv1alpha1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1alpha1"
)

type testCase struct {
	reason string
	args   args
	want   want
}

type args struct {
	url      string
	key      string
	version  string
	jsonpath string
	response string
}

type want struct {
	path   string
	err    string
	result string
}

func TestWebhookGetSecret(t *testing.T) {
	testCase := &testCase{
		reason: "get secret simple",
		args: args{
			url:      "/api/getsecret?id={{ .remoteRef.key }}&version={{ .remoteRef.version }}",
			key:      "testkey",
			version:  "1",
			jsonpath: "$.result.thesecret",
			response: `{"result":{"thesecret":"secret-value"}}`,
		},
		want: want{
			path:   "/api/getsecret?id=testkey&version=1",
			err:    "",
			result: "secret-value",
		},
	}
	runTestCase(testCase, t)
}

func runTestCase(tc *testCase, t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.String() != tc.want.path {
			t.Errorf("%s: unexpected api path: %s, expected %s", tc.reason, req.URL.String(), tc.want.path)
		}
		rw.Write([]byte(tc.args.response))
	}))
	defer ts.Close()

	testStore := makeClusterSecretStore(ts.URL+tc.args.url, tc.args.jsonpath)
	testProv := &Provider{}
	client, err := testProv.NewClient(context.Background(), testStore, nil, "testnamespace")
	if err != nil {
		t.Errorf("%s: error creating client: %s", tc.reason, err.Error())
		return
	}

	testRef := esv1alpha1.ExternalSecretDataRemoteRef{
		Key:     tc.args.key,
		Version: tc.args.version,
	}
	secret, err := client.GetSecret(context.Background(), testRef)
	if !errorContains(err, tc.want.err) {
		t.Errorf("%s: unexpected error: %s (expected '%s')", tc.reason, err.Error(), tc.want.err)
	}
	if err == nil && string(secret) != tc.want.result {
		t.Errorf("%s:unexpected response: %s (expected '%s')", tc.reason, secret, tc.want.result)
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
