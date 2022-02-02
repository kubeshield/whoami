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
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"kmodules.xyz/resource-metadata/apis/meta/v1alpha1"
	"kmodules.xyz/resource-metadata/hub/menuoutlines"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func RenderAccordionMenu(kc client.Client, disco discovery.ServerResourcesInterface, menuName string) (*v1alpha1.Menu, error) {
	mo, err := menuoutlines.LoadByName(menuName)
	if err != nil {
		return nil, err
	}

	out, err := GenerateMenuItems(kc, disco)
	if err != nil {
		return nil, err
	}

	menu := v1alpha1.Menu{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       v1alpha1.ResourceKindMenu,
		},
		Home:     mo.Home,
		Sections: nil,
	}

	for _, so := range mo.Sections {
		sec := v1alpha1.MenuSection{
			MenuSectionInfo: so.MenuSectionInfo,
		}
		if sec.AutoDiscoverAPIGroup != "" {
			kinds := out[sec.AutoDiscoverAPIGroup]
			for _, item := range kinds {
				sec.Items = append(sec.Items, *item) // variants
			}
			sort.Slice(sec.Items, func(i, j int) bool {
				return sec.Items[i].Name < sec.Items[j].Name
			})
		} else {
			items := make([]v1alpha1.MenuItem, 0, len(so.Items))
			for _, item := range so.Items {
				mi := v1alpha1.MenuItem{
					Name:       item.Name,
					Path:       item.Path,
					Resource:   nil,
					Missing:    true,
					Required:   item.Required,
					LayoutName: item.LayoutName,
					Icons:      item.Icons,
					Installer:  nil,
				}

				if item.Type != nil {
					if generated, ok := getMenuItem(out, *item.Type); ok {
						mi.Resource = generated.Resource
						mi.Missing = false
						mi.Installer = generated.Installer
						if mi.LayoutName == "" {
							mi.LayoutName = generated.LayoutName
						}
					}
				}
				items = append(items, mi)
			}
			sec.Items = items
		}

		if len(sec.Items) > 0 {
			menu.Sections = append(menu.Sections, &sec)
		}
	}

	return &menu, nil
}
