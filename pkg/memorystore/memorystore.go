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

package memorystore

import (
	"context"
	"fmt"
	"github.com/onmetal/matryoshka/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type key struct {
	GroupKind schema.GroupKind
	ObjectKey client.ObjectKey
}

type Store struct {
	scheme  *runtime.Scheme
	entries map[key]client.Object
}

func (s *Store) keyOf(obj client.Object) (key, error) {
	gvk, err := apiutil.GVKForObject(obj, s.scheme)
	if err != nil {
		return key{}, err
	}

	return key{
		GroupKind: gvk.GroupKind(),
		ObjectKey: client.ObjectKeyFromObject(obj),
	}, nil
}

func validateClientCreateOptions(o *client.CreateOptions) error {
	if o.Raw != nil {
		return fmt.Errorf("raw is not supported")
	}
	if o.DryRun != nil {
		return fmt.Errorf("dry run is not supported")
	}
	return nil
}

func (s *Store) Create(_ context.Context, obj client.Object, opts ...client.CreateOption) error {
	o := &client.CreateOptions{}
	o.ApplyOptions(opts)
	if err := validateClientCreateOptions(o); err != nil {
		return err
	}

	key, err := s.keyOf(obj)
	if err != nil {
		return err
	}

	if _, ok := s.entries[key]; ok {
		return apierrors.NewAlreadyExists(schema.GroupResource{
			Group:    key.GroupKind.Group,
			Resource: key.GroupKind.Kind,
		}, key.ObjectKey.String())
	}

	s.entries[key] = obj
	return nil
}

func (s *Store) Get(_ context.Context, objectKey client.ObjectKey, obj client.Object) error {
	key, err := s.keyOf(obj)
	if err != nil {
		return err
	}
	key.ObjectKey = objectKey

	v, ok := s.entries[key]
	if !ok {
		return apierrors.NewNotFound(schema.GroupResource{
			Group:    key.GroupKind.Group,
			Resource: key.GroupKind.Kind,
		}, objectKey.String())
	}

	return s.scheme.Convert(v, obj, nil)
}

func (s *Store) Objects() []client.Object {
	res := make([]client.Object, 0, len(s.entries))
	for _, obj := range s.entries {
		res = append(res, obj)
	}
	return res
}

func (s *Store) GroupKinds() []schema.GroupKind {
	gks := make(map[schema.GroupKind]struct{})
	var res []schema.GroupKind
	for k := range s.entries {
		if _, ok := gks[k.GroupKind]; !ok {
			gks[k.GroupKind] = struct{}{}
			res = append(res, k.GroupKind)
		}
	}
	return res
}

func (s *Store) GroupKindObjects(gk schema.GroupKind) []client.Object {
	var objs []client.Object
	for key, obj := range s.entries {
		if key.GroupKind == gk {
			objs = append(objs, obj)
		}
	}
	return objs
}

func validateClientListOptions(opts *client.ListOptions) error {
	if opts.Raw != nil {
		return fmt.Errorf("raw is not supported")
	}
	if opts.Continue != "" {
		return fmt.Errorf("continue is not supported")
	}
	if opts.Limit != 0 {
		return fmt.Errorf("limit is not supported")
	}
	if opts.FieldSelector != nil {
		return fmt.Errorf("field selector is not supported")
	}
	return nil
}

func objectMatchesClientListOptions(obj client.Object, opts *client.ListOptions) bool {
	if opts.Namespace != "" && opts.Namespace != obj.GetNamespace() {
		return false
	}
	if opts.LabelSelector != nil && !opts.LabelSelector.Matches(labels.Set(obj.GetLabels())) {
		return false
	}
	return true
}

func (s *Store) List(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
	o := &client.ListOptions{}
	o.ApplyOptions(opts)
	if err := validateClientListOptions(o); err != nil {
		return err
	}

	gvk, err := utils.GVKForList(s.scheme, list)
	if err != nil {
		return err
	}

	var res []client.Object
	for k, obj := range s.entries {
		if k.GroupKind == gvk.GroupKind() && objectMatchesClientListOptions(obj, o) {
			res = append(res, obj)
		}
	}
	return utils.ConvertAndSetList(s.scheme, list, res)
}

func (s *Store) Status() client.StatusWriter {
	return s
}

func validateClientDeleteOptions(opts *client.DeleteOptions) error {
	if opts.DryRun != nil {
		return fmt.Errorf("dry run is not supported")
	}
	if opts.GracePeriodSeconds != nil {
		return fmt.Errorf("grace period seconds is not supported")
	}
	if opts.Preconditions != nil {
		return fmt.Errorf("preconditions is not supported")
	}
	if opts.PropagationPolicy != nil {
		return fmt.Errorf("propagation policy is not supported")
	}
	return nil
}

func (s *Store) Delete(_ context.Context, obj client.Object, opts ...client.DeleteOption) error {
	o := &client.DeleteOptions{}
	o.ApplyOptions(opts)
	if err := validateClientDeleteOptions(o); err != nil {
		return err
	}

	key, err := s.keyOf(obj)
	if err != nil {
		return err
	}

	if _, ok := s.entries[key]; !ok {
		return apierrors.NewNotFound(schema.GroupResource{
			Group:    key.GroupKind.Group,
			Resource: key.GroupKind.Kind,
		}, key.ObjectKey.String())
	}
	delete(s.entries, key)
	return nil
}

func validateClientDeleteAllOfOptions(o *client.DeleteAllOfOptions) error {
	if err := validateClientListOptions(&o.ListOptions); err != nil {
		return err
	}
	if err := validateClientDeleteOptions(&o.DeleteOptions); err != nil {
		return err
	}
	return nil
}

func (s *Store) DeleteAllOf(_ context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	o := &client.DeleteAllOfOptions{}
	o.ApplyOptions(opts)
	if err := validateClientDeleteAllOfOptions(o); err != nil {
		return err
	}

	gvk, err := apiutil.GVKForObject(obj, s.scheme)
	if err != nil {
		return err
	}

	for k, obj := range s.entries {
		if k.GroupKind == gvk.GroupKind() && objectMatchesClientListOptions(obj, &o.ListOptions) {
			delete(s.entries, k)
		}
	}
	return nil
}

func validateClientUpdateOptions(opts *client.UpdateOptions) error {
	if opts.DryRun != nil {
		return fmt.Errorf("dry run is not supported")
	}
	if opts.Raw != nil {
		return fmt.Errorf("raw is not supported")
	}
	if opts.FieldManager != "" {
		return fmt.Errorf("field manager is not supported")
	}
	return nil
}

func (s *Store) Update(_ context.Context, obj client.Object, opts ...client.UpdateOption) error {
	o := &client.UpdateOptions{}
	o.ApplyOptions(opts)
	if err := validateClientUpdateOptions(o); err != nil {
		return err
	}

	key, err := s.keyOf(obj)
	if err != nil {
		return err
	}

	if _, ok := s.entries[key]; !ok {
		return apierrors.NewNotFound(schema.GroupResource{
			Group:    key.GroupKind.Group,
			Resource: key.GroupKind.Kind,
		}, key.ObjectKey.String())
	}
	s.entries[key] = obj
	return nil
}

func (s *Store) Patch(_ context.Context, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
	return fmt.Errorf("patch is not supported")
}

func (s *Store) Scheme() *runtime.Scheme {
	return s.scheme
}

func (s *Store) RESTMapper() meta.RESTMapper {
	return nil
}

func New(scheme *runtime.Scheme) *Store {
	return &Store{
		scheme:  scheme,
		entries: make(map[key]client.Object),
	}
}
