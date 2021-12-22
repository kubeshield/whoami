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

package graph

import (
	"sync"

	"gomodules.xyz/sets"
	ksets "gomodules.xyz/sets/kubernetes"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiv1 "kmodules.xyz/client-go/api/v1"
	"kmodules.xyz/resource-metadata/apis/meta/v1alpha1"
	"kmodules.xyz/resource-metadata/hub"
	setx "kmodules.xyz/resource-metadata/pkg/utils/sets"
)

type ObjectGraph struct {
	m     sync.RWMutex
	edges map[apiv1.OID]map[v1alpha1.EdgeLabel]setx.OID // oid -> label -> edges
	ids   map[apiv1.OID]map[v1alpha1.EdgeLabel]setx.OID // oid -> label -> edges
}

func (g *ObjectGraph) Update(src apiv1.OID, connsPerLabel map[v1alpha1.EdgeLabel]setx.OID) {
	g.m.Lock()
	defer g.m.Unlock()

	for lbl, conns := range connsPerLabel {

		if oldConnsPerLabel, ok := g.ids[src]; ok {
			if oldConns, ok := oldConnsPerLabel[lbl]; ok {
				if oldConns.Difference(conns).Len() == 0 {
					return
				}

				g.edges[src][lbl].Delete(oldConns.UnsortedList()...)
				for dst := range oldConns {
					g.edges[dst][lbl].Delete(src)
				}
			}
		}

		if _, ok := g.edges[src]; !ok {
			g.edges[src] = map[v1alpha1.EdgeLabel]setx.OID{}
		}
		if _, ok := g.edges[src][lbl]; !ok {
			g.edges[src][lbl] = setx.NewOID()
		}
		g.edges[src][lbl].Insert(conns.UnsortedList()...)

		for dst := range conns {
			if _, ok := g.edges[dst]; !ok {
				g.edges[dst] = map[v1alpha1.EdgeLabel]setx.OID{}
			}
			if _, ok := g.edges[dst][lbl]; !ok {
				g.edges[dst][lbl] = setx.NewOID()
			}
			g.edges[dst][lbl].Insert(src)
		}
	}

	g.ids[src] = connsPerLabel
}

func (g *ObjectGraph) Links(oid *apiv1.ObjectID, edgeLabel v1alpha1.EdgeLabel) (map[metav1.GroupKind][]apiv1.ObjectID, error) {
	g.m.RLock()
	defer g.m.RUnlock()

	if edgeLabel == v1alpha1.EdgeOffshoot {
		return g.links(oid, nil, edgeLabel)
	}

	src := oid.OID()
	offshoots := g.connectedOIDs([]apiv1.OID{src}, v1alpha1.EdgeOffshoot)
	offshoots.Delete(src)
	return g.links(oid, offshoots.UnsortedList(), edgeLabel)
}

func (g *ObjectGraph) links(oid *apiv1.ObjectID, seeds []apiv1.OID, edgeLabel v1alpha1.EdgeLabel) (map[metav1.GroupKind][]apiv1.ObjectID, error) {
	src := oid.OID()
	links := g.connectedOIDs(append([]apiv1.OID{src}, seeds...), edgeLabel)
	links.Delete(src)

	result := map[metav1.GroupKind][]apiv1.ObjectID{}
	for v := range links {
		id, err := apiv1.ParseObjectID(v)
		if err != nil {
			return nil, err
		}
		gk := id.MetaGroupKind()
		result[gk] = append(result[gk], *id)
	}
	return result, nil
}

func (g *ObjectGraph) connectedOIDs(idsToProcess []apiv1.OID, edgeLabel v1alpha1.EdgeLabel) setx.OID {
	links := setx.NewOID()
	var x apiv1.OID
	for len(idsToProcess) > 0 {
		x, idsToProcess = idsToProcess[0], idsToProcess[1:]
		links.Insert(x)

		var edges setx.OID
		if edgedPerLabel, ok := g.edges[x]; ok {
			edges = edgedPerLabel[edgeLabel]
		}
		for id := range edges {
			if !links.Has(id) {
				idsToProcess = append(idsToProcess, id)
			}
		}
	}
	return links
}

type objectEdge struct {
	Source apiv1.OID
	Target apiv1.OID
}

func ResourceGraph(mapper meta.RESTMapper, src apiv1.ObjectID) (*v1alpha1.ResourceGraphResponse, error) {
	objGraph.m.RLock()
	defer objGraph.m.RUnlock()

	return objGraph.resourceGraph(mapper, src)
}

func (g *ObjectGraph) resourceGraph(mapper meta.RESTMapper, src apiv1.ObjectID) (*v1alpha1.ResourceGraphResponse, error) {
	connections := map[objectEdge]sets.String{}

	offshoots := g.connectedEdges([]apiv1.OID{src.OID()}, v1alpha1.EdgeOffshoot, connections).List()
	for _, label := range hub.ListEdgeLabels() {
		if label != v1alpha1.EdgeOffshoot {
			g.connectedEdges(offshoots, label, connections)
		}
	}

	objIDs := ksets.NewGroupKind()
	var objID *apiv1.ObjectID
	for e := range connections {
		objID, _ = apiv1.ParseObjectID(e.Source)
		objIDs.Insert(objID.GroupKind())
		objID, _ = apiv1.ParseObjectID(e.Target)
		objIDs.Insert(objID.GroupKind())
	}
	gks := objIDs.List()

	resp := v1alpha1.ResourceGraphResponse{
		Resources:   make([]apiv1.ResourceID, len(gks)),
		Connections: make([]v1alpha1.ObjectConnection, 0, len(connections)),
	}

	gkMap := map[schema.GroupKind]int{}
	for idx, gk := range gks {
		gkMap[gk] = idx

		mapping, err := mapper.RESTMapping(gk)
		if err != nil {
			return nil, err
		}
		scope := apiv1.ClusterScoped
		if mapping.Scope == meta.RESTScopeNamespace {
			scope = apiv1.NamespaceScoped
		}
		resp.Resources[idx] = apiv1.ResourceID{
			Group:   mapping.GroupVersionKind.Group,
			Version: mapping.GroupVersionKind.Version,
			Name:    mapping.Resource.Resource,
			Kind:    mapping.GroupVersionKind.Kind,
			Scope:   scope,
		}
	}

	for e, labels := range connections {
		src, _ := apiv1.ParseObjectID(e.Source)
		target, _ := apiv1.ParseObjectID(e.Target)

		resp.Connections = append(resp.Connections, v1alpha1.ObjectConnection{
			Source: v1alpha1.ObjectPointer{
				ResourceID: gkMap[src.GroupKind()],
				Namespace:  src.Namespace,
				Name:       src.Name,
			},
			Target: v1alpha1.ObjectPointer{
				ResourceID: gkMap[target.GroupKind()],
				Namespace:  target.Namespace,
				Name:       target.Name,
			},
			Labels: labels.List(),
		})
	}
	return &resp, nil
}

func (g *ObjectGraph) connectedEdges(idsToProcess []apiv1.OID, edgeLabel v1alpha1.EdgeLabel, connections map[objectEdge]sets.String) setx.OID {
	links := setx.NewOID()
	var x apiv1.OID
	for len(idsToProcess) > 0 {
		x, idsToProcess = idsToProcess[0], idsToProcess[1:]
		links.Insert(x)

		var edges setx.OID
		if edgedPerLabel, ok := g.edges[x]; ok {
			edges = edgedPerLabel[edgeLabel]
		}
		for id := range edges {
			var key objectEdge
			if x < id {
				key = objectEdge{
					Source: x,
					Target: id,
				}
			} else {
				key = objectEdge{
					Source: id,
					Target: x,
				}
			}
			if _, ok := connections[key]; !ok {
				connections[key] = sets.NewString()
			}
			connections[key].Insert(string(edgeLabel))

			if !links.Has(id) {
				idsToProcess = append(idsToProcess, id)
			}
		}
	}
	return links
}
