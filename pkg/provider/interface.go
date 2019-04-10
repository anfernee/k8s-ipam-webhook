package provider

import (
	"errors"

	// TODO(anfernee): Remove the dependency on v1beta1
	ipamv1beta1 "github.com/anfernee/k8s-ipam-webhook/pkg/apis/ipam/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IPAMContext ...
type IPAMContext struct {
	Interface *ipamv1beta1.InterfaceSpec

	// Object runtime.Object
}

// IPAMProvider ...
type IPAMProvider interface {
	// Allocate tries to allocate an IP address from an IPAM provider
	Allocate(ctx IPAMContext) (ipamv1beta1.IPConfig, error)

	// Release tries to release an address to an IPAM provider
	Release(ctx IPAMContext, ipConfig ipamv1beta1.IPConfig) error
}

// InjectClient ...
type InjectClient interface {
	SetClient(client.Client)
}

// Register ...
func Register(gvk schema.GroupVersionKind, provider IPAMProvider) {
	if _, ok := Providers[gvk]; ok {
		panic(gvk.String() + " already registered")
	}
	Providers[gvk] = provider
}

// Providers ...
var Providers map[schema.GroupVersionKind]IPAMProvider

func init() {
	Providers = make(map[schema.GroupVersionKind]IPAMProvider)
}

var (
	// ErrProviderNotReady ...
	ErrProviderNotReady = errors.New("provider not ready")

	// ErrNoAddressAvailable ...
	ErrNoAddressAvailable = errors.New("no address available")

	// ErrBadRelease ...
	ErrBadRelease = errors.New("bad release")
)
