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
// Author Adam Janikowski
//

package inspector

import (
	"github.com/arangodb/kube-arangodb/pkg/util/errors"
	"github.com/arangodb/kube-arangodb/pkg/util/k8sutil/inspector/pod"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func (i *inspector) IteratePods(action pod.Action, filters ...pod.Filter) error {
	for _, pod := range i.Pods() {
		if err := i.iteratePod(pod, action, filters...); err != nil {
			return err
		}
	}
	return nil
}

func (i *inspector) iteratePod(pod *core.Pod, action pod.Action, filters ...pod.Filter) error {
	for _, filter := range filters {
		if !filter(pod) {
			return nil
		}
	}

	return action(pod)
}

func (i *inspector) Pods() []*core.Pod {
	i.lock.Lock()
	defer i.lock.Unlock()

	var r []*core.Pod
	for _, pod := range i.pods {
		r = append(r, pod)
	}

	return r
}

func (i *inspector) Pod(name string) (*core.Pod, bool) {
	i.lock.Lock()
	defer i.lock.Unlock()

	pod, ok := i.pods[name]
	if !ok {
		return nil, false
	}

	return pod, true
}

func podsToMap(k kubernetes.Interface, namespace string) (map[string]*core.Pod, error) {
	pods, err := getPods(k, namespace, "")
	if err != nil {
		return nil, err
	}

	podMap := map[string]*core.Pod{}

	for _, pod := range pods {
		_, exists := podMap[pod.GetName()]
		if exists {
			return nil, errors.Newf("Pod %s already exists in map, error received", pod.GetName())
		}

		podMap[pod.GetName()] = podPointer(pod)
	}

	return podMap, nil
}

func podPointer(pod core.Pod) *core.Pod {
	return &pod
}

func getPods(k kubernetes.Interface, namespace, cont string) ([]core.Pod, error) {
	pods, err := k.CoreV1().Pods(namespace).List(meta.ListOptions{
		Limit:    128,
		Continue: cont,
	})

	if err != nil {
		return nil, err
	}

	if pods.Continue != "" {
		nextPodsLayer, err := getPods(k, namespace, pods.Continue)
		if err != nil {
			return nil, err
		}

		return append(pods.Items, nextPodsLayer...), nil
	}

	return pods.Items, nil
}

func FilterPodsByLabels(labels map[string]string) pod.Filter {
	return func(pod *core.Pod) bool {
		for key, value := range labels {
			v, ok := pod.Labels[key]
			if !ok {
				return false
			}

			if v != value {
				return false
			}
		}

		return true
	}
}
