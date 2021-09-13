package multigetter

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RequestAccumulator struct {
	err     error
	objects *Objects
}

func (o *RequestAccumulator) Add(key client.ObjectKey, obj client.Object) error {
	obj.SetNamespace(key.Namespace)
	obj.SetName(key.Name)
	return o.objects.Set(obj)
}

func (o *RequestAccumulator) Has(key client.ObjectKey, obj client.Object) (bool, error) {
	return o.objects.Has(key, obj)
}

func (o *RequestAccumulator) Resolve(ctx context.Context, mg *Multigetter) (*Objects, error) {
	requests := make([]Request, 0, o.objects.Len())
	for _, obj := range o.objects.List() {
		requests = append(requests, Request{
			Key:    client.ObjectKeyFromObject(obj),
			Object: obj,
		})
	}

	if err := mg.MultiGet(ctx, requests...); err != nil {
		return nil, fmt.Errorf("error multi-getting: %w", err)
	}
	return o.objects, nil
}

type RequestAccumulatorOptions struct {
	Scheme *runtime.Scheme
}

func (o *RequestAccumulatorOptions) Validate() error {
	if o.Scheme == nil {
		return fmt.Errorf("scheme needs to be set")
	}
	return nil
}

func NewAccumulator(scheme *runtime.Scheme) *RequestAccumulator {
	return &RequestAccumulator{
		objects: NewObjects(scheme),
	}
}
