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

package reconcile

import (
	"context"
	"encoding/base64"

	"github.com/arangodb/kube-arangodb/pkg/util/errors"

	"github.com/arangodb/kube-arangodb/pkg/util/constants"

	api "github.com/arangodb/kube-arangodb/pkg/apis/deployment/v1"
	"github.com/arangodb/kube-arangodb/pkg/deployment/patch"
	"github.com/arangodb/kube-arangodb/pkg/deployment/pod"
	"github.com/arangodb/kube-arangodb/pkg/util"
	"github.com/arangodb/kube-arangodb/pkg/util/k8sutil"
	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/types"
)

func init() {
	registerAction(api.ActionTypeJWTSetActive, newJWTSetActive)
}

func newJWTSetActive(log zerolog.Logger, action api.Action, actionCtx ActionContext) Action {
	a := &jwtSetActiveAction{}

	a.actionImpl = newActionImplDefRef(log, action, actionCtx, defaultTimeout)

	return a
}

type jwtSetActiveAction struct {
	actionImpl

	actionEmptyCheckProgress
}

func (a *jwtSetActiveAction) Start(ctx context.Context) (bool, error) {
	folder, err := ensureJWTFolderSupportFromAction(a.actionCtx)
	if err != nil {
		a.log.Error().Err(err).Msgf("Action not supported")
		return true, nil
	}

	if !folder {
		a.log.Error().Msgf("Action not supported")
		return true, nil
	}

	toActiveChecksum, exists := a.action.Params[checksum]
	if !exists {
		a.log.Warn().Msgf("Key %s is missing in action", checksum)
		return true, nil
	}

	f, ok := a.actionCtx.GetCachedStatus().Secret(pod.JWTSecretFolder(a.actionCtx.GetName()))
	if !ok {
		a.log.Error().Msgf("Unable to get JWT folder info")
		return true, nil
	}

	toActiveData, toActivePresent := f.Data[toActiveChecksum]
	if !toActivePresent {
		a.log.Error().Msgf("JWT key which is desired to be active is not anymore in secret")
		return true, nil
	}

	activeKeyData, active := f.Data[pod.ActiveJWTKey]
	tokenKeyData, token := f.Data[constants.SecretKeyToken]

	if util.SHA256(activeKeyData) == toActiveChecksum && util.SHA256(activeKeyData) == util.SHA256(tokenKeyData) {
		a.log.Info().Msgf("Desired JWT is already active")
		return true, nil
	}

	p := patch.NewPatch()
	path := patch.NewPath("data", pod.ActiveJWTKey)
	if !active {
		p.ItemAdd(path, base64.StdEncoding.EncodeToString(toActiveData))
	} else {
		p.ItemReplace(path, base64.StdEncoding.EncodeToString(toActiveData))
	}

	path = patch.NewPath("data", constants.SecretKeyToken)
	if !token {
		p.ItemAdd(path, base64.StdEncoding.EncodeToString(toActiveData))
	} else {
		p.ItemReplace(path, base64.StdEncoding.EncodeToString(toActiveData))
	}

	patch, err := p.Marshal()
	if err != nil {
		a.log.Error().Err(err).Msgf("Unable to encrypt patch")
		return true, nil
	}

	_, err = a.actionCtx.SecretsInterface().Patch(pod.JWTSecretFolder(a.actionCtx.GetName()), types.JSONPatchType, patch)
	if err != nil {
		if !k8sutil.IsInvalid(err) {
			return false, errors.Wrapf(err, "Unable to update secret: %s", pod.JWTSecretFolder(a.actionCtx.GetName()))
		}
	}

	return true, nil
}
