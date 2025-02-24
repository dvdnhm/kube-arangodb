//
// DISCLAIMER
//
// Copyright 2020 ArangoDB GmbH, Cologne, Germany
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Copyright holder is ArangoDB GmbH, Cologne, Germany
//
// Author Ewout Prangsma
//

package storage

import (
	"github.com/arangodb/kube-arangodb/pkg/util/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/arangodb/kube-arangodb/pkg/apis/storage/v1alpha"
	"github.com/arangodb/kube-arangodb/pkg/storage/provisioner"
	"github.com/arangodb/kube-arangodb/pkg/util/k8sutil"
)

// ensureProvisionerService ensures that a service is created for accessing the
// provisioners.
func (ls *LocalStorage) ensureProvisionerService(apiObject *api.ArangoLocalStorage) error {
	labels := k8sutil.LabelsForLocalStorage(apiObject.GetName(), roleProvisioner)
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   apiObject.GetName(),
			Labels: labels,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				v1.ServicePort{
					Name:     "provisioner",
					Protocol: v1.ProtocolTCP,
					Port:     provisioner.DefaultPort,
				},
			},
			Selector: labels,
		},
	}
	svc.SetOwnerReferences(append(svc.GetOwnerReferences(), apiObject.AsOwner()))
	ns := ls.config.Namespace
	if _, err := ls.deps.KubeCli.CoreV1().Services(ns).Create(svc); err != nil && !k8sutil.IsAlreadyExists(err) {
		return errors.WithStack(err)
	}
	return nil
}
