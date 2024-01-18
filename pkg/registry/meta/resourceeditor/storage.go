/*
Copyright AppsCode Inc. and Contributors

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

package resourceeditor

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	kerr "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/rest"
	meta_util "kmodules.xyz/client-go/meta"
	"kmodules.xyz/resource-metadata/apis/meta"
	rsapi "kmodules.xyz/resource-metadata/apis/meta/v1alpha1"
	uiapi "kmodules.xyz/resource-metadata/apis/ui/v1alpha1"
	"kmodules.xyz/resource-metadata/hub/resourceeditors"
)

type Storage struct {
	convertor rest.TableConvertor
}

var (
	_ rest.GroupVersionKindProvider = &Storage{}
	_ rest.Scoper                   = &Storage{}
	_ rest.Storage                  = &Storage{}
	_ rest.Getter                   = &Storage{}
	_ rest.Lister                   = &Storage{}
	_ rest.SingularNameProvider     = &Storage{}
)

func NewStorage() *Storage {
	return &Storage{
		convertor: rest.NewDefaultTableConvertor(schema.GroupResource{
			Group:    rsapi.SchemeGroupVersion.Group,
			Resource: uiapi.ResourceResourceEditors,
		}),
	}
}

func (r *Storage) GroupVersionKind(_ schema.GroupVersion) schema.GroupVersionKind {
	return rsapi.SchemeGroupVersion.WithKind(uiapi.ResourceKindResourceEditor)
}

func (r *Storage) NamespaceScoped() bool {
	return false
}

func (r *Storage) GetSingularName() string {
	return strings.ToLower(uiapi.ResourceKindResourceEditor)
}

// Getter
func (r *Storage) New() runtime.Object {
	return &uiapi.ResourceEditor{}
}

func (r *Storage) Destroy() {}

func (r *Storage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	obj, err := resourceeditors.LoadByName(name)
	if err != nil {
		return nil, kerr.NewNotFound(schema.GroupResource{Group: meta.GroupName, Resource: uiapi.ResourceKindResourceEditor}, name)
	}
	return complete(obj.DeepCopy()), err
}

// Lister
func (r *Storage) NewList() runtime.Object {
	return &uiapi.ResourceEditorList{}
}

func (r *Storage) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	if options.FieldSelector != nil && !options.FieldSelector.Empty() {
		return nil, kerr.NewBadRequest("fieldSelector is not a supported")
	}

	objs := resourceeditors.List()

	if options.Continue != "" {
		start, err := strconv.Atoi(options.Continue)
		if err != nil {
			return nil, kerr.NewBadRequest(fmt.Sprintf("invalid continue option, err:%v", err))
		}
		if start > len(objs) {
			return r.NewList(), nil
		}
		objs = objs[start:]
	}
	if options.Limit > 0 && int64(len(objs)) > options.Limit {
		objs = objs[:options.Limit]
	}

	items := make([]uiapi.ResourceEditor, 0, len(objs))
	for _, obj := range objs {
		if options.LabelSelector != nil && !options.LabelSelector.Matches(labels.Set(obj.GetLabels())) {
			continue
		}
		items = append(items, *complete(obj.DeepCopy()))
	}

	return &uiapi.ResourceEditorList{Items: items}, nil
}

var podNamespace = meta_util.PodNamespace()

func complete(obj *uiapi.ResourceEditor) *uiapi.ResourceEditor {
	if obj.Spec.UI == nil {
		return obj
	}
	if obj.Spec.UI.Editor != nil {
		if obj.Spec.UI.Editor.SourceRef.Namespace == "" {
			obj.Spec.UI.Editor.SourceRef.Namespace = podNamespace
		}
	}
	if obj.Spec.UI.Options != nil {
		if obj.Spec.UI.Options.SourceRef.Namespace == "" {
			obj.Spec.UI.Options.SourceRef.Namespace = podNamespace
		}
	}
	return obj
}

func (r *Storage) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return r.convertor.ConvertToTable(ctx, object, tableOptions)
}
