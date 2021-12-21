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
	"testing"
	"fmt"
	"path/filepath"
	"runtime"
	"reflect"
	"net/http"
	"net/http/httptest"
	esv1alpha1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// assert fails the test if the condition is false.
func assert(tb testing.TB, condition bool, msg string, v ...interface{}) {
	if !condition {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: "+msg+"\033[39m\n\n", append([]interface{}{filepath.Base(file), line}, v...)...)
		tb.FailNow()
	}
}

// ok fails the test if an err is not nil.
func ok(tb testing.TB, err error) {
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: unexpected error: %s\033[39m\n\n", filepath.Base(file), line, err.Error())
		tb.FailNow()
	}
}

// equals fails the test if exp is not equal to act.
func equals(tb testing.TB, exp, act interface{}) {
	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\texp: %#v\n\n\tgot: %#v\033[39m\n\n", filepath.Base(file), line, exp, act)
		tb.FailNow()
	}
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
					URL:     url,
					Headers: map[string]string{
						"Content-Type":"application.json",
						"X-SecretKey":"{{ .remoteRef.key }}",
					},
					Result: esv1alpha1.WebhookResult{
						JSONPath: jsonpath,
					},
				},
			},
		},
	}
}

func TestWebhookGetSecret(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		equals(t, req.URL.String(), "/getsecret")
		rw.Write([]byte(`{"testresponse":{"password":"test-secret"}}`))
	}))
	defer ts.Close()

	testStore := makeClusterSecretStore(ts.URL+"/getsecret", "$.testresponse.password")
	testProv := &Provider{}
	client, err := testProv.NewClient(context.Background(), testStore, nil, "testnamespace")
	ok(t, err)

	testRef := esv1alpha1.ExternalSecretDataRemoteRef{ }
	secret, err := client.GetSecret(context.Background(), testRef)
	ok(t, err)
	equals(t, string(secret), "test-secret")
}
