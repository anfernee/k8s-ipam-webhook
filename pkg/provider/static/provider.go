package static

import (
	"context"

	ipamv1beta1 "github.com/anfernee/k8s-ipam-webhook/pkg/apis/ipam/v1beta1"
	"github.com/anfernee/k8s-ipam-webhook/pkg/provider"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	gvk := ipamv1beta1.SchemeGroupVersion.WithKind("IPPool")
	provider.Register(gvk, New())
}

type staticProvider struct {
	client.Client

	// sync.Mutex // TODO(anfernee): synchronize allocate/release
}

// New creates an IPAMProvider instance
func New() provider.IPAMProvider {
	return &staticProvider{}
}

func (p *staticProvider) Allocate(ctx provider.IPAMContext) (ipamv1beta1.IPConfig, error) {
	var result ipamv1beta1.IPConfig
	var ipPool ipamv1beta1.IPPool

	if !p.Ready() {
		return result, provider.ErrProviderNotReady
	}

	nsname := types.NamespacedName{
		Namespace: ctx.Interface.IPAMPool.Namespace,
		Name:      ctx.Interface.IPAMPool.Name,
	}

	if err := p.Get(context.Background(), nsname, &ipPool); err != nil {
		return result, err
	}

	if len(ipPool.Spec.ReservedAddresses) == 0 {
		return result, provider.ErrNoAddressAvailable
	}

	address := ipPool.Spec.ReservedAddresses[0]
	ipPool.Spec.ReservedAddresses = ipPool.Spec.ReservedAddresses[1:]
	ipPool.Status.AllocatedAddresses = append(ipPool.Status.AllocatedAddresses, address)

	// TODO(anfernee):
	// - Can I update spec/status at one call?
	// - Retry or not?
	if err := p.Update(context.Background(), &ipPool); err != nil {
		return result, err
	}

	result.IPv4 = address.IPv4
	result.Gateway = address.Gateway
	result.Netmask = address.Netmask
	result.DNS = ipPool.Spec.DNS
	result.NTP = ipPool.Spec.NTP

	return result, nil
}

func (p *staticProvider) Release(ctx provider.IPAMContext, ipConfig ipamv1beta1.IPConfig) error {
	var ipPool ipamv1beta1.IPPool

	if !p.Ready() {
		return provider.ErrProviderNotReady
	}

	nsname := types.NamespacedName{
		Namespace: ctx.Interface.IPAMPool.Namespace,
		Name:      ctx.Interface.IPAMPool.Name,
	}

	if err := p.Get(context.Background(), nsname, &ipPool); err != nil {
		return err
	}

	found := false
	for i, address := range ipPool.Status.AllocatedAddresses {
		if address.IPv4 == ipConfig.IPv4 {
			ipPool.Status.AllocatedAddresses = append(ipPool.Status.AllocatedAddresses[:i], ipPool.Status.AllocatedAddresses[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return provider.ErrBadRelease
	}

	address := ipamv1beta1.Address{
		IPv4:    ipConfig.IPv4,
		Gateway: ipConfig.Gateway,
		Netmask: ipConfig.Netmask,
	}
	// TODO(anfernee): Sort it?
	ipPool.Spec.ReservedAddresses = append(ipPool.Spec.ReservedAddresses, address)

	// TODO(anfernee):
	// - Can I update spec/status at one call?
	// - Retry or not?
	return p.Update(context.Background(), &ipPool)
}

func (p *staticProvider) Ready() bool {
	return p.Client != nil
}

func (p *staticProvider) SetClient(clt client.Client) {
	p.Client = clt
}
