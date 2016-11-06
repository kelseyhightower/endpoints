// Copyright 2016 Google Inc. All Rights Reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package endpoints

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	endpointsPath      = "/api/v1/namespaces/%s/endpoints/%s"
	endpointsWatchPath = "/api/v1/watch/namespaces/%s/endpoints/%s"
)

const (
	DefaultAPIHost      = "127.0.0.1:8001"
	DefaultNamespace    = "default"
	DefaultSyncInterval = 30 * time.Second
	DefaultRetryDelay   = 5 * time.Second
)

var (
	// ErrNoEndpoints is returned by EndpointsManager.Next calls
	// when a named service has no backends.
	ErrNoEndpoints = errors.New("endpoints: no endpoints available")

	// ErrNotExist is returned when attempting to lookup a service
	// that does not exist in the request namespace.
	ErrNotExist = errors.New("endpoints: service does not exist")

	// ErrMissingServiceName is returned by New when attempting
	// to initilizate an EndpointsManager with a config that
	// contains a missing or blank service name.
	ErrMissingServiceName = errors.New("endpoints: missing service name")
)

// A Config structure is used to configure a EndpointsManager.
type Config struct {
	// The http.Client used to perform requests to the Kubernetes API.
	// If nil, http.DefaultClient is used.
	Client *http.Client

	// ErrorLog specifies an optional logger for errors
	// that occur when attempting to sync endpoints.
	// If nil, logging goes to os.Stderr via the log package's
	// standard logger.
	ErrorLog *log.Logger

	// The Kubernetes namespace to search for services.
	// If empty, DefaultNamespace is used.
	Namespace string

	// RetryDelay is the amount of time to wait between API
	// calls after an error occurs. If empty, DefaultRetryDelay
	// is used.
	RetryDelay time.Duration

	// The Kubernetes service to monitor.
	Service string

	// SyncInterval is the amount of time between request to
	// reconcile the list of endpoint backends from Kubernetes.
	SyncInterval time.Duration
}

// EndpointsManager represents a Kubernetes endpoints round-robin load balancer.
type EndpointsManager struct {
	apiHost      string
	client       *http.Client
	errorLog     *log.Logger
	namespace    string
	retryDelay   time.Duration
	service      string
	syncInterval time.Duration
	quit         chan struct{}
	wg           *sync.WaitGroup

	mu              *sync.Mutex // protects currentEndpoint and endpoints
	currentEndpoint int
	endpoints       []Endpoint
}

// New returns a new endpoints load balancer.
func New(config *Config) (*EndpointsManager, error) {
	if config.Service == "" {
		return nil, ErrMissingServiceName
	}

	config.setDefaults()

	em := &EndpointsManager{
		apiHost:      DefaultAPIHost,
		client:       config.Client,
		mu:           &sync.Mutex{},
		namespace:    config.Namespace,
		retryDelay:   config.RetryDelay,
		service:      config.Service,
		syncInterval: config.SyncInterval,
		quit:         make(chan struct{}),
		wg:           &sync.WaitGroup{},
	}

	err := em.SyncEndpoints()
	if err != nil {
		return nil, err
	}

	// Start watching for changes to the endpoint.
	em.wg.Add(1)
	go em.watchEndpoints()
	// Start reconciliation loop.
	em.wg.Add(1)
	go em.reconcile()

	return em, nil
}

func (c *Config) setDefaults() {
	if c.Client == nil {
		c.Client = http.DefaultClient
	}
	if c.Namespace == "" {
		c.Namespace = DefaultNamespace
	}
	if c.RetryDelay <= 0 {
		c.RetryDelay = DefaultRetryDelay
	}
	if c.SyncInterval <= 0 {
		c.SyncInterval = DefaultSyncInterval
	}
}

// Next returns the next Kubernetes endpoint.
func (em *EndpointsManager) Next() (Endpoint, error) {
	em.mu.Lock()
	defer em.mu.Unlock()
	if len(em.endpoints) <= 0 {
		return Endpoint{}, ErrNoEndpoints
	}
	if em.currentEndpoint >= len(em.endpoints) {
		em.currentEndpoint = 0
	}
	endpoint := em.endpoints[em.currentEndpoint]
	em.currentEndpoint++
	return endpoint, nil
}

// update updates the endpoints cache.
func (em *EndpointsManager) update(endpoints []Endpoint) {
	em.mu.Lock()
	em.endpoints = endpoints
	em.mu.Unlock()
}

func (em *EndpointsManager) reconcile() {
	defer em.wg.Done()
	for {
		select {
		case <-time.After(em.syncInterval):
			err := em.SyncEndpoints()
			if err != nil {
				em.errorLog.Println(err)
			}
		case <-em.quit:
			return
		}
	}
}

// Stop terminates the endpoints watcher.
func (em *EndpointsManager) Shutdown() error {
	close(em.quit)
	em.wg.Wait()
	return nil
}

func (em *EndpointsManager) SyncEndpoints() error {
	var eps Endpoints
	r, err := em.get(context.TODO(), fmt.Sprintf(endpointsPath, em.namespace, em.service))
	if err != nil {
		return err
	}
	defer r.Close()

	err = json.NewDecoder(r).Decode(&eps)
	if err != nil {
		return err
	}

	em.update(formatEndpoints(eps))
	return nil
}

func (em *EndpointsManager) watchEndpoints() {
	defer em.wg.Done()

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go em.watch(ctx, &wg)

	<-em.quit
	cancel()
	wg.Wait()
}

func (em *EndpointsManager) watch(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	path := fmt.Sprintf(endpointsWatchPath, em.namespace, em.service)

	for {
		r, err := em.get(ctx, path)
		if ctx.Err() == context.Canceled {
			return
		}
		if err != nil {
			em.errorLog.Println(err)
			time.Sleep(em.retryDelay)
			continue
		}

		// endpoint watches return a stream of JSON objects which
		// must be processed one at a time to ensure consistency.
		decoder := json.NewDecoder(r)
		for {
			if ctx.Err() == context.Canceled {
				r.Close()
				return
			}

			var o Object
			err := decoder.Decode(&o)
			if err != nil {
				em.errorLog.Println(err)
				r.Close()
				break
			}
			if o.Type == "ERROR" {
				em.errorLog.Println(o.Object.Message)
				r.Close()
				break
			}
			em.update(formatEndpoints(o.Object))
		}
	}
}

func (em *EndpointsManager) get(ctx context.Context, path string) (io.ReadCloser, error) {
	r := &http.Request{
		Header: make(http.Header),
		Method: http.MethodGet,
		URL: &url.URL{
			Host:   em.apiHost,
			Path:   path,
			Scheme: "http",
		},
	}
	r.Header.Set("Accept", "application/json, */*")

	resp, err := em.client.Do(r.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 404 {
		return nil, ErrNotExist
	}
	if resp.StatusCode != 200 {
		return nil, errors.New("error non 200 reponse: " + resp.Status)
	}
	return resp.Body, nil
}

func formatEndpoints(endpoints Endpoints) []Endpoint {
	eps := make([]Endpoint, 0)
	if len(endpoints.Subsets) == 0 {
		return eps
	}

	port := ""
	ports := make(map[string]string)
	if len(endpoints.Subsets[0].Ports) > 0 {
		port = strconv.FormatInt(int64(endpoints.Subsets[0].Ports[0].Port), 10)
		for _, p := range endpoints.Subsets[0].Ports {
			if p.Name != "" {
				ports[p.Name] = strconv.FormatInt(int64(p.Port), 10)
			}
		}
	}

	for _, address := range endpoints.Subsets[0].Addresses {
		ep := Endpoint{
			Host:  address.IP,
			Port:  port,
			Ports: ports,
		}
		eps = append(eps, ep)
	}
	return eps
}
