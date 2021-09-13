package multigetter

import (
	"context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type objectsKey struct {
	GVK       schema.GroupVersionKind
	ObjectKey client.ObjectKey
}

type Objects struct {
	scheme      *runtime.Scheme
	keyToObject map[objectsKey]client.Object
}

func (o *Objects) Len() int {
	return len(o.keyToObject)
}

func (o *Objects) List() []client.Object {
	res := make([]client.Object, 0, len(o.keyToObject))
	for _, obj := range o.keyToObject {
		res = append(res, obj)
	}
	return res
}

func (o *Objects) Set(obj client.Object) error {
	gvk, err := apiutil.GVKForObject(obj, o.scheme)
	if err != nil {
		return err
	}

	o.keyToObject[objectsKey{gvk, client.ObjectKeyFromObject(obj)}] = obj
	return nil
}

func (o *Objects) Has(key client.ObjectKey, obj client.Object) (bool, error) {
	gvk, err := apiutil.GVKForObject(obj, o.scheme)
	if err != nil {
		return false, err
	}

	_, ok := o.keyToObject[objectsKey{gvk, key}]
	return ok, nil
}

func (o *Objects) Get(key client.ObjectKey, obj client.Object) error {
	gvk, err := apiutil.GVKForObject(obj, o.scheme)
	if err != nil {
		return err
	}

	v, ok := o.keyToObject[objectsKey{gvk, key}]
	if !ok {
		return apierrors.NewNotFound(schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}, key.String())
	}

	return o.scheme.Convert(v, obj, nil)
}

func NewObjects(scheme *runtime.Scheme) *Objects {
	return &Objects{
		scheme:      scheme,
		keyToObject: make(map[objectsKey]client.Object),
	}
}

type objectsClientOverlay struct {
	client.Client
	objs *Objects
}

func (o *objectsClientOverlay) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	if err := o.objs.Get(key, obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		return o.Client.Get(ctx, key, obj)
	}
	return nil
}

func ObjectsClientOverlay(c client.Client, objs *Objects) client.Client {
	return &objectsClientOverlay{c, objs}
}
