// Copyright 2021 OnMetal authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package multigetter

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type Multigetter struct {
	client    client.Client
	threshold *Threshold
}

type Request struct {
	Key    client.ObjectKey
	Object client.Object
}

func RequestFromObject(obj client.Object) Request {
	return Request{
		Key:    client.ObjectKeyFromObject(obj),
		Object: obj,
	}
}

func RequestsFromObjects(objs []client.Object) []Request {
	if objs == nil {
		return nil
	}
	res := make([]Request, 0, len(objs))
	for _, obj := range objs {
		res = append(res, RequestFromObject(obj))
	}
	return res
}

type group struct {
	GVK      schema.GroupVersionKind
	Requests []Request
}

var DefaultThreshold = &Threshold{
	Global: ListMoreThanTwo,
}

type Threshold struct {
	Global ThresholdValue
	ByGVK  map[schema.GroupVersionKind]ThresholdValue
}

type ThresholdValue int

const (
	NeverList       ThresholdValue = -1
	AlwaysList      ThresholdValue = 0
	ListMoreThanOne ThresholdValue = 2
	ListMoreThanTwo ThresholdValue = 3
)

func (v ThresholdValue) Validate() error {
	if int(v) < -1 {
		return fmt.Errorf("invalid threshold value %d: cannot be smaller than %d (never list)",
			int(v), NeverList)
	}
	return nil
}

func ListMoreThan(n int) ThresholdValue {
	v := ThresholdValue(n)
	if err := v.Validate(); err != nil {
		panic(err)
	}
	return v
}

func (t *Threshold) DeepCopy() *Threshold {
	byGVK := make(map[schema.GroupVersionKind]ThresholdValue, len(t.ByGVK))
	for gvk, v := range t.ByGVK {
		byGVK[gvk] = v
	}
	return &Threshold{t.Global, byGVK}
}

func (t *Threshold) Validate() error {
	if err := t.Global.Validate(); err != nil {
		return fmt.Errorf("invalid global threshold: %w", err)
	}
	for gvk, v := range t.ByGVK {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("gvk %s invalid threshold: %w", gvk, err)
		}
	}
	return nil
}

func (t *Threshold) NeverList() bool {
	if t.Global == NeverList {
		for _, v := range t.ByGVK {
			if v > NeverList {
				return false
			}
		}
		return true
	}
	return false
}

func (t *Threshold) Get(gvk schema.GroupVersionKind) ThresholdValue {
	if v, ok := t.ByGVK[gvk]; ok {
		return v
	}
	return t.Global
}

func (m *Multigetter) shouldList(group group) bool {
	if len(group.Requests) <= 1 {
		return false
	}
	threshold := m.threshold.Get(group.GVK)
	if threshold == NeverList {
		return false
	}
	return int(threshold) <= len(group.Requests)
}

func (m *Multigetter) groupRequests(requests []Request) ([]group, error) {
	gvkToRequests := make(map[schema.GroupVersionKind][]Request)
	for i, request := range requests {
		gvk, err := apiutil.GVKForObject(request.Object, m.client.Scheme())
		if err != nil {
			return nil, fmt.Errorf("[request %d]: could not determine gvk: %w", i, err)
		}

		gvkToRequests[gvk] = append(gvkToRequests[gvk], request)
	}

	groups := make([]group, 0, len(gvkToRequests))
	for gvk, requests := range gvkToRequests {
		groups = append(groups, group{
			GVK:      gvk,
			Requests: requests,
		})
	}
	return groups, nil
}

func (m *Multigetter) resolveGroups(ctx context.Context, groups []group) error {
	for _, group := range groups {
		if m.shouldList(group) {
			if err := m.listGroup(ctx, group); err != nil {
				return err
			}
		} else {
			if err := m.getGroup(ctx, group); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Multigetter) listGroup(ctx context.Context, group group) error {
	list := &unstructured.UnstructuredList{}
	list.SetAPIVersion(group.GVK.GroupVersion().String())
	list.SetKind(group.GVK.Kind)

	if err := m.client.List(ctx, list); err != nil {
		return fmt.Errorf("error listing gvk %s: %w", group.GVK, err)
	}

	keyToObject := make(map[client.ObjectKey]client.Object)
	for _, request := range group.Requests {
		keyToObject[request.Key] = request.Object
	}

	for _, obj := range list.Items {
		key := client.ObjectKeyFromObject(&obj)
		if dst, ok := keyToObject[key]; ok {
			if err := m.client.Scheme().Convert(&obj, dst, nil); err != nil {
				return fmt.Errorf("error converting %s %s: %w", group.GVK, key, err)
			}
		}
	}

	return nil
}

func (m *Multigetter) getGroup(ctx context.Context, group group) error {
	for _, request := range group.Requests {
		if err := m.client.Get(ctx, request.Key, request.Object); err != nil {
			return fmt.Errorf("error getting %v %s: %w", group.GVK, request.Key, err)
		}
	}
	return nil
}

func (m *Multigetter) MultiGet(ctx context.Context, requests ...Request) error {
	groups, err := m.groupRequests(requests)
	if err != nil {
		return fmt.Errorf("error grouping requests: %w", err)
	}

	return m.resolveGroups(ctx, groups)
}

type Options struct {
	Client    client.Client
	Threshold *Threshold
}

func (o *Options) SetDefaults() {
	if o.Threshold == nil {
		o.Threshold = DefaultThreshold
	}
}

func (o *Options) Validate() error {
	if o.Client == nil {
		return fmt.Errorf("client needs to be set")
	}
	if o.Threshold == nil {
		return fmt.Errorf("threshold needs to be set")
	}
	if err := o.Threshold.Validate(); err != nil {
		return fmt.Errorf("invalid threshold: %w", err)
	}
	return nil
}

func New(opts Options) (*Multigetter, error) {
	opts.SetDefaults()
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	return &Multigetter{
		client:    opts.Client,
		threshold: opts.Threshold.DeepCopy(),
	}, nil
}
