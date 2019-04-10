package static

import (
	"github.com/anfernee/k8s-ipam-webhook/pkg/apis"
	"k8s.io/client-go/kubernetes/scheme"
)

// func TestMain(m *testing.M) {
func init() {
	apis.AddToScheme(scheme.Scheme)
}
