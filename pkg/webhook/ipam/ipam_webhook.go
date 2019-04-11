/*
Copyright 2018 The Kubernetes Authors.
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

package ipam

import (
	"context"
	"net/http"

	ipamv1beta1 "github.com/anfernee/k8s-ipam-webhook/pkg/apis/ipam/v1beta1"
	"github.com/anfernee/k8s-ipam-webhook/pkg/provider"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

// IPAMAllocator annotates Pods
type IPAMAllocator struct {
	client  client.Client
	decoder types.Decoder
}

// FIXME: Not used
func Add(mgr manager.Manager) error {
	return nil
}

// Implement admission.Handler so the controller can handle admission request.
var _ admission.Handler = &IPAMAllocator{}

// IPAMAllocator adds an annotation to every incoming machine.
func (a *IPAMAllocator) Handle(ctx context.Context, req types.Request) types.Response {
	log := logf.Log.WithName("entrypoint")

	log.Info("receive webhook request")

	machine := &ipamv1beta1.Machine{}

	err := a.decoder.Decode(req, machine)
	if err != nil {
		return admission.ErrorResponse(http.StatusBadRequest, err)
	}
	copy := machine.DeepCopy()

	// IP is already configured
	if copy.Spec.Interface.IPConfig != nil {
		return admission.PatchResponse(machine, copy)
	}

	// No IPAM pool specified
	if machine.Spec.Interface.IPAMPool == nil {
		// TODO: more effective way?
		return admission.PatchResponse(machine, copy)
	}

	gvk := machine.Spec.Interface.IPAMPool.GroupVersionKind()
	log.Info("ippool gvk: " + gvk.String())

	if _, ok := provider.Providers[gvk]; !ok {
		log.Info("gvk not found")
		return admission.PatchResponse(machine, copy)
	}

	ipam := provider.Providers[gvk]

	switch req.AdmissionRequest.Operation {
	case admissionv1beta1.Create:
		ipconfig, err := ipam.Allocate(provider.IPAMContext{Interface: &machine.Spec.Interface})
		if err != nil {
			return admission.ErrorResponse(http.StatusBadRequest, err)
		}
		copy.Spec.Interface.IPConfig = &ipconfig
	case admissionv1beta1.Delete:
		// TODO(anfernee): Use finalizer
		if copy.Spec.Interface.IPConfig != nil {
			err := ipam.Release(provider.IPAMContext{Interface: &machine.Spec.Interface}, *copy.Spec.Interface.IPConfig)
			if err != nil {
				return admission.ErrorResponse(http.StatusBadRequest, err)
			}
		}
	}

	return admission.PatchResponse(machine, copy)
}

// IPAMAllocator implements inject.Client.
// A client will be automatically injected.
var _ inject.Client = &IPAMAllocator{}

// InjectClient injects the client.
func (v *IPAMAllocator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// IPAMAllocator implements inject.Decoder.
// A decoder will be automatically injected.
var _ inject.Decoder = &IPAMAllocator{}

// InjectDecoder injects the decoder.
func (v *IPAMAllocator) InjectDecoder(d types.Decoder) error {
	v.decoder = d
	return nil
}
