package static

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"testing"

	ipamv1beta1 "github.com/anfernee/k8s-ipam-webhook/pkg/apis/ipam/v1beta1"
	"github.com/anfernee/k8s-ipam-webhook/pkg/provider"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAllocateRelease(t *testing.T) {
	tests := []struct {
		desc      string
		provider  *staticProvider
		ipPool    *ipamv1beta1.IPPool
		ipPoolEnd *ipamv1beta1.IPPool
		steps     []step
	}{
		{
			desc:      "no addresses",
			provider:  New().(*staticProvider),
			ipPool:    newBuilder("ns", "n").reserved().allocated().pool,
			ipPoolEnd: newBuilder("ns", "n").reserved().allocated().pool,
			steps: []step{
				allocate(fromIpamPool("ns", "n")).expect(hasError(provider.ErrNoAddressAvailable)),
			},
		},
		{
			desc:      "valid allocate",
			provider:  New().(*staticProvider),
			ipPool:    newBuilder("ns", "n").reserved("1.2.3.2", "1.2.3.3").pool,
			ipPoolEnd: newBuilder("ns", "n").reserved().allocated("1.2.3.2", "1.2.3.3").pool,
			steps: []step{
				allocate(fromIpamPool("ns", "n")).expect(hasAddress("1.2.3.2")),
				allocate(fromIpamPool("ns", "n")).expect(hasAddress("1.2.3.3")),
				allocate(fromIpamPool("ns", "n")).expect(hasError(provider.ErrNoAddressAvailable)),
			},
		},
	}

	for _, test := range tests {
		client := fake.NewFakeClient(test.ipPool)
		test.provider.SetClient(client)

		for _, step := range test.steps {
			if _, err := step(test.provider); err != nil {
				t.Error(err)
				break
			}
		}

		ipPool := &ipamv1beta1.IPPool{}
		ipPool.Spec.ReservedAddresses = []ipamv1beta1.Address{}
		ipPool.Status.AllocatedAddresses = []ipamv1beta1.Address{}
		if err := client.Get(context.Background(), types.NamespacedName{
			Namespace: "ns",
			Name:      "n",
		}, ipPool); err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(ipPool.Spec.ReservedAddresses, test.ipPoolEnd.Spec.ReservedAddresses) {
			t.Errorf("got ippool.Spec.ReservedAddresses %+v; expect %+v", ipPool.Spec.ReservedAddresses, test.ipPoolEnd.Spec.ReservedAddresses)
		}
		if !reflect.DeepEqual(ipPool.Status.AllocatedAddresses, test.ipPoolEnd.Status.AllocatedAddresses) {
			t.Errorf("got ippool.Spec.ReservedAddresses %+v; expect %+v", ipPool.Status.AllocatedAddresses, test.ipPoolEnd.Status.AllocatedAddresses)
		}
	}
}

type result struct {
	ipconfig ipamv1beta1.IPConfig
	err      error
}

type step func(*staticProvider) (result, error)

type expectation func(result, error) error

func allocate(ctx provider.IPAMContext) step {
	return func(p *staticProvider) (r result, err error) {
		r.ipconfig, r.err = p.Allocate(ctx)
		return r, r.err
	}
}

func (s step) expect(exp expectation) step {
	return func(p *staticProvider) (r result, err error) {
		r, err = s(p)
		return r, exp(r, err)
	}
}

func hasAddress(ip string) expectation {
	return func(r result, err error) error {
		if err != nil {
			return err
		}
		if r.ipconfig.IPv4 != ip {
			return fmt.Errorf("got IP %v; expect %v", r.ipconfig.IPv4, ip)
		}
		return nil
	}
}

func hasError(expErr error) expectation {
	return func(r result, err error) error {
		if err != expErr {
			return fmt.Errorf("got error %v; expect %v", err, expErr)
		}
		return nil
	}
}

func fromIpamPool(ns, n string) provider.IPAMContext {
	return provider.IPAMContext{
		Interface: &ipamv1beta1.InterfaceSpec{
			IPAMPool: &corev1.ObjectReference{
				Namespace: ns,
				Name:      n,
			},
		},
	}
}

type builder struct {
	pool *ipamv1beta1.IPPool
}

func newBuilder(ns, n string) *builder {
	pool := &ipamv1beta1.IPPool{}
	pool.Namespace = ns
	pool.Name = n

	return &builder{
		pool: pool,
	}
}

func defaultGateway(ip string) string {
	ipp := net.ParseIP(ip)
	ipp[3] = 1
	return ipp.String()
}

func addresses(ips []string) []ipamv1beta1.Address {
	result := []ipamv1beta1.Address{}
	for _, ip := range ips {
		result = append(result, ipamv1beta1.Address{
			IPv4:    ip,
			Netmask: "255.255.255.0",
			Gateway: defaultGateway(ip),
		})
	}
	return result
}

func (b *builder) reserved(ips ...string) *builder {
	b.pool.Spec.ReservedAddresses = addresses(ips)
	return b
}

func (b *builder) allocated(ips ...string) *builder {
	b.pool.Status.AllocatedAddresses = addresses(ips)
	return b
}
