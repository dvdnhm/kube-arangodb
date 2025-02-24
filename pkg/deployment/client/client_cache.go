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

package client

import (
	"context"
	"net"
	"strconv"
	"sync"

	"github.com/arangodb/kube-arangodb/pkg/util/errors"

	"github.com/arangodb/go-driver/agency"
	"github.com/arangodb/kube-arangodb/pkg/util/arangod/conn"
	"github.com/arangodb/kube-arangodb/pkg/util/k8sutil"

	driver "github.com/arangodb/go-driver"
	api "github.com/arangodb/kube-arangodb/pkg/apis/deployment/v1"
)

type Cache interface {
	GetAuth() conn.Auth

	Get(ctx context.Context, group api.ServerGroup, id string) (driver.Client, error)
	GetDatabase(ctx context.Context) (driver.Client, error)
	GetAgency(ctx context.Context) (agency.Agency, error)
}

func NewClientCache(apiObjectGetter func() *api.ArangoDeployment, factory conn.Factory) Cache {
	return &cache{
		apiObjectGetter: apiObjectGetter,
		factory:         factory,
	}
}

type cache struct {
	mutex           sync.Mutex
	apiObjectGetter func() *api.ArangoDeployment

	factory conn.Factory
}

func (cc *cache) extendHost(host string) string {
	scheme := "http"
	if cc.apiObjectGetter().Spec.TLS.IsSecure() {
		scheme = "https"
	}

	return scheme + "://" + net.JoinHostPort(host, strconv.Itoa(k8sutil.ArangoPort))
}

func (cc *cache) getClient(ctx context.Context, group api.ServerGroup, id string) (driver.Client, error) {
	m, _, _ := cc.apiObjectGetter().Status.Members.ElementByID(id)

	c, err := cc.factory.Client(cc.extendHost(m.GetEndpoint(k8sutil.CreatePodDNSName(cc.apiObjectGetter(), group.AsRole(), id))))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return c, nil
}

func (cc *cache) get(ctx context.Context, group api.ServerGroup, id string) (driver.Client, error) {
	client, err := cc.getClient(ctx, group, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if _, err := client.Version(ctx); err == nil {
		return client, nil
	} else if driver.IsUnauthorized(err) {
		return cc.getClient(ctx, group, id)
	} else {
		return client, nil
	}
}

// Get a cached client for the given ID in the given group, creating one
// if needed.
func (cc *cache) Get(ctx context.Context, group api.ServerGroup, id string) (driver.Client, error) {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()

	return cc.get(ctx, group, id)
}

func (cc cache) GetAuth() conn.Auth {
	return cc.factory.GetAuth()
}

func (cc *cache) getDatabaseClient() (driver.Client, error) {
	c, err := cc.factory.Client(cc.extendHost(k8sutil.CreateDatabaseClientServiceDNSName(cc.apiObjectGetter())))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return c, nil
}

func (cc *cache) getDatabase(ctx context.Context) (driver.Client, error) {
	client, err := cc.getDatabaseClient()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if _, err := client.Version(ctx); err == nil {
		return client, nil
	} else if driver.IsUnauthorized(err) {
		return cc.getDatabaseClient()
	} else {
		return client, nil
	}
}

// GetDatabase returns a cached client for the entire database (cluster coordinators or single server),
// creating one if needed.
func (cc *cache) GetDatabase(ctx context.Context) (driver.Client, error) {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()

	return cc.getDatabase(ctx)
}

func (cc *cache) getAgencyClient() (agency.Agency, error) {
	// Not found, create a new client
	var dnsNames []string
	for _, m := range cc.apiObjectGetter().Status.Members.Agents {
		dnsNames = append(dnsNames, cc.extendHost(m.GetEndpoint(k8sutil.CreatePodDNSName(cc.apiObjectGetter(), api.ServerGroupAgents.AsRole(), m.ID))))
	}

	if len(dnsNames) == 0 {
		return nil, errors.Newf("There is no DNS Name")
	}

	c, err := cc.factory.Agency(dnsNames...)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return c, nil
}

// GetDatabase returns a cached client for the agency
func (cc *cache) GetAgency(ctx context.Context) (agency.Agency, error) {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()

	return cc.getAgencyClient()
}
