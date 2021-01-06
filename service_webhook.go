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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/aojea/clusterip-webhook/pkg/allocator"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ClusterIPAllocator allocates Services Cluster IPs
type ClusterIPAllocator struct {
	Client    client.Client
	decoder   *admission.Decoder
	allocator *allocator.Range
}

// NewClusterIPAllocator create a ClusterIPAllocator
func NewClusterIPAllocator(client client.Client, serviceSubnet string) (*ClusterIPAllocator, error) {
	// TODO parametrize
	_, subnet, err := net.ParseCIDR(serviceSubnet)
	if err != nil {
		return &ClusterIPAllocator{}, err
	}
	r, err := allocator.NewAllocatorCIDRRange(subnet, client)
	if err != nil {
		return &ClusterIPAllocator{}, err
	}
	return &ClusterIPAllocator{
		Client:    client,
		allocator: r,
	}, nil
}

// Handle allocates a ClusterIP to services that needs an allocation
func (a *ClusterIPAllocator) Handle(ctx context.Context, req admission.Request) admission.Response {
	svc := &corev1.Service{}

	err := a.decoder.Decode(req, svc)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	klog.Infof("Handle service %v", svc)
	// clusterIP is set, validate and allocate it in the API IPRange object
	if len(svc.Spec.ClusterIP) > 0 && svc.Spec.ClusterIP != corev1.ClusterIPNone {
		ip := net.ParseIP(svc.Spec.ClusterIP)
		if ip == nil {
			return admission.Errored(http.StatusInternalServerError, fmt.Errorf("invalid IP address %s", svc.Spec.ClusterIP))
		}
		err = a.allocator.Allocate(ip)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		klog.Infof("IP allocated %s", svc.Spec.ClusterIP)
		return admission.Allowed("")
	}
	// ClusterIP is empty, we need to allocate one
	alloc, err := a.allocator.AllocateNext()
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	svc.Spec.ClusterIP = alloc.String()
	klog.Infof("IP allocated %s", svc.Spec.ClusterIP)
	marshaledSvc, err := json.Marshal(svc)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledSvc)
}

// ClusterIPAllocator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (a *ClusterIPAllocator) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
