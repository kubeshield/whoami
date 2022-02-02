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

package menu

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	kmapi "kmodules.xyz/client-go/api/v1"
	"kmodules.xyz/resource-metadata/apis/meta/v1alpha1"
	"kmodules.xyz/resource-metadata/hub"
	"kmodules.xyz/resource-metadata/hub/resourceoutlines"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GenerateMenuItems(kc client.Client, disco discovery.ServerResourcesInterface) (map[string]map[string]*v1alpha1.MenuItem, error) {
	reg := hub.NewRegistryOfKnownResources()

	rsLists, err := disco.ServerPreferredResources()
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		return nil, err
	}

	// [group][Kind] => MenuItem
	out := map[string]map[string]*v1alpha1.MenuItem{}
	for _, rsList := range rsLists {
		gv, err := schema.ParseGroupVersion(rsList.GroupVersion)
		if err != nil {
			return nil, err
		}

		for _, rs := range rsList.APIResources {
			// skip sub resource
			if strings.ContainsRune(rs.Name, '/') {
				continue
			}

			// if resource can't be listed or read (get) or only view type skip it
			verbs := sets.NewString(rs.Verbs...)
			if !verbs.HasAll("list", "get", "watch", "create") {
				continue
			}

			scope := kmapi.ClusterScoped
			if rs.Namespaced {
				scope = kmapi.NamespaceScoped
			}
			rid := kmapi.ResourceID{
				Group:   gv.Group,
				Version: gv.Version,
				Name:    rs.Name,
				Kind:    rs.Kind,
				Scope:   scope,
			}
			gvr := rid.GroupVersionResource()

			me := v1alpha1.MenuItem{
				Name:       rid.Kind,
				Path:       "",
				Resource:   &rid,
				Missing:    false,
				Required:   false,
				LayoutName: resourceoutlines.DefaultLayoutName(gvr),
				// Icons:    rd.Spec.Icons,
				// Installer:  rd.Spec.Installer,
			}
			if rd, err := reg.LoadByGVR(gvr); err == nil {
				me.Icons = rd.Spec.Icons
			}
			if rd, ok := LoadResourceEditor(kc, gvr); ok {
				me.Installer = rd.Spec.Installer
			}

			if _, ok := out[gv.Group]; !ok {
				out[gv.Group] = map[string]*v1alpha1.MenuItem{}
			}
			out[gv.Group][rs.Kind] = &me // variants
		}
	}

	return out, nil
}

func getMenuItem(out map[string]map[string]*v1alpha1.MenuItem, gk metav1.GroupKind) (*v1alpha1.MenuItem, bool) {
	m, ok := out[gk.Group]
	if !ok {
		return nil, false
	}
	item, ok := m[gk.Kind]
	return item, ok
}
