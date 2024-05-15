/*
Copyright AppsCode Inc. and Contributors.

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

package addofflinelicense

import (
	"context"
	"strings"

	licenseapi "kubeops.dev/ui-server/apis/offline/v1alpha1"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/util/cert"
	cg "kmodules.xyz/client-go/client"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	LicenseSecretName      = "license-proxyserver-licenses"
	LicenseSecretNamespace = "kubeops"
)

type Storage struct {
	kc client.Client
}

var (
	_ rest.GroupVersionKindProvider = &Storage{}
	_ rest.Scoper                   = &Storage{}
	_ rest.Storage                  = &Storage{}
	_ rest.Creater                  = &Storage{}
	_ rest.SingularNameProvider     = &Storage{}
)

func NewStorage(kc client.Client) *Storage {
	return &Storage{
		kc: kc,
	}
}

func (r *Storage) GroupVersionKind(_ schema.GroupVersion) schema.GroupVersionKind {
	return licenseapi.SchemeGroupVersion.WithKind(licenseapi.ResourceKindAddOfflineLicense)
}

func (r *Storage) NamespaceScoped() bool {
	return true
}

func (r *Storage) GetSingularName() string {
	return strings.ToLower(licenseapi.ResourceKindAddOfflineLicense)
}

func (r *Storage) New() runtime.Object {
	return &licenseapi.AddOfflineLicense{}
}

func (r *Storage) Destroy() {}

func (r *Storage) Create(ctx context.Context, obj runtime.Object, _ rest.ValidateObjectFunc, _ *metav1.CreateOptions) (runtime.Object, error) {
	ns, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, apierrors.NewBadRequest("missing namespace")
	}

	in := obj.(*licenseapi.AddOfflineLicense)
	if in.Request == nil {
		return nil, apierrors.NewBadRequest("missing apirequest")
	}
	req := in.Request

	var resp licenseapi.AddOfflineLicenseResponse
	if req.License != "" {
		licenseSecret := v1.Secret{}
		err := r.kc.Get(ctx, types.NamespacedName{Name: LicenseSecretName, Namespace: ns}, &licenseSecret)
		if err != nil && apierrors.IsNotFound(err) {
			productKey, err := getProductKey([]byte(req.License))
			if err != nil {
				return nil, err
			}

			licenseSecret = v1.Secret{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      LicenseSecretName,
					Namespace: LicenseSecretNamespace,
				},
				Data: map[string][]byte{
					productKey: []byte(req.License),
				},
			}
			if err = r.kc.Create(ctx, &licenseSecret); err != nil {
				return nil, err
			}

			return in, nil

		} else if err != nil {
			return nil, err
		}

		productKey, err := getProductKey([]byte(req.License))
		if err != nil {
			return nil, err
		}
		licenseSecret.Data[productKey] = []byte(req.License)

		_, err = cg.CreateOrPatch(ctx, r.kc, &licenseSecret, func(obj client.Object, createOp bool) client.Object {
			in := obj.(*v1.Secret)
			in.Data = licenseSecret.Data
			return in
		})
		if err != nil {
			return nil, err
		}
	}
	in.Response = &resp

	return in, nil
}

func getProductKey(lic []byte) (string, error) {
	certs, err := cert.ParseCertsPEM(lic)
	if err != nil {
		return "", err
	}
	return certs[0].Subject.OrganizationalUnit[0], nil
}
