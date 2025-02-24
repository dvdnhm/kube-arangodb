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
)

// inspectPVCs queries all PVC's and checks if there is a need to
// build new persistent volumes.
// Returns the PVC's that need a volume.
func (ls *LocalStorage) inspectPVCs() ([]v1.PersistentVolumeClaim, error) {
	ns := ls.apiObject.GetNamespace()
	list, err := ls.deps.KubeCli.CoreV1().PersistentVolumeClaims(ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	spec := ls.apiObject.Spec
	var result []v1.PersistentVolumeClaim
	for _, pvc := range list.Items {
		if !pvcMatchesStorageClass(pvc, spec.StorageClass.Name, spec.StorageClass.IsDefault) {
			continue
		}
		if !pvcNeedsVolume(pvc) {
			continue
		}
		result = append(result, pvc)
	}
	return result, nil
}

// pvcMatchesStorageClass checks if the given pvc requests a volume
// of the given storage class.
func pvcMatchesStorageClass(pvc v1.PersistentVolumeClaim, storageClassName string, isDefault bool) bool {
	scn := pvc.Spec.StorageClassName
	if scn == nil {
		// No storage class specified, default is used
		return isDefault
	}
	return *scn == storageClassName
}

// pvcNeedsVolume checks if the given pvc is in need of a persistent volume.
func pvcNeedsVolume(pvc v1.PersistentVolumeClaim) bool {
	return pvc.Status.Phase == v1.ClaimPending
}
