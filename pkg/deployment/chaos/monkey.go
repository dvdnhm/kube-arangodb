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

package chaos

import (
	"math/rand"
	"time"

	"github.com/arangodb/kube-arangodb/pkg/util/errors"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Monkey is the service that introduces chaos in the deployment
// if allowed and enabled.
type Monkey struct {
	log     zerolog.Logger
	context Context
}

// NewMonkey creates a new chaos monkey with given context.
func NewMonkey(log zerolog.Logger, context Context) *Monkey {
	log = log.With().Str("component", "chaos-monkey").Logger()
	return &Monkey{
		log:     log,
		context: context,
	}
}

// Run the monkey until the given channel is closed.
func (m Monkey) Run(stopCh <-chan struct{}) {
	for {
		spec := m.context.GetSpec()
		if spec.Chaos.IsEnabled() {
			// Gamble to set if we must introduce chaos
			chance := float64(spec.Chaos.GetKillPodProbability()) / 100.0
			if rand.Float64() < chance {
				// Let's introduce pod chaos
				if err := m.killRandomPod(); err != nil {
					log.Info().Err(err).Msg("Failed to kill random pod")
				}
			}
		}

		select {
		case <-time.After(spec.Chaos.GetInterval()):
			// Continue
		case <-stopCh:
			// We're done
			return
		}
	}
}

// killRandomPod fetches all owned pods and tries to kill one.
func (m Monkey) killRandomPod() error {
	pods, err := m.context.GetOwnedPods()
	if err != nil {
		return errors.WithStack(err)
	}
	if len(pods) <= 1 {
		// Not enough pods
		return nil
	}
	p := pods[rand.Intn(len(pods))]
	m.log.Info().Str("pod-name", p.GetName()).Msg("Killing pod")
	if err := m.context.DeletePod(p.GetName()); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
