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

package elb

import (
	"errors"
	"log"
	"sync"
	"time"
)

var (
	defaultNamespace = "default"
)

var ErrNoEndpoints = errors.New("no endpoints available")
var ErrMissingServiceName = errors.New("missing service name")

// ELB represents a Kubernetes endpoints round-robin load balancer.
type ELB struct {
	namespace       string
	service         string
	quit            chan struct{}
	mu              *sync.Mutex
	currentEndpoint int
	endpoints       []Endpoint
}

// New returns a new endpoints load balancer.
func New(namespace, service string) (*ELB, error) {
	if service == "" {
		return nil, ErrMissingServiceName
	}
	if namespace == "" {
		namespace = defaultNamespace
	}

	e := &ELB{
		namespace:       namespace,
		service:         service,
		quit:            make(chan struct{}),
		mu:              &sync.Mutex{},
		currentEndpoint: 0,
	}

	endpoints, err := getEndpoints(namespace, service)
	if err != nil {
		return nil, err
	}
	e.endpoints = endpoints

	// Start watching for changes to the endpoint.
	go e.watch()
	// Start reconciliation loop.
	go e.reconcile()

	return e, nil
}

// Next returns the next Kubernetes endpoint.
func (elb *ELB) Next() (Endpoint, error) {
	elb.mu.Lock()
	defer elb.mu.Unlock()
	if len(elb.endpoints) <= 0 {
		return Endpoint{}, ErrNoEndpoints
	}
	if elb.currentEndpoint >= len(elb.endpoints) {
		elb.currentEndpoint = 0
	}
	endpoint := elb.endpoints[elb.currentEndpoint]
	elb.currentEndpoint++
	return endpoint, nil
}

// update updates the endpoints cache.
func (elb *ELB) update(endpoints []Endpoint) {
	elb.mu.Lock()
	elb.endpoints = endpoints
	elb.mu.Unlock()
}

// watch starts the endpoints watcher.
func (elb *ELB) watch() {
	done := make(chan struct{})
	endpointsc, errc := watchEndpoints(elb.namespace, elb.service, done)

	for {
		select {
		case err := <-errc:
			log.Println(err)
		case endpoints := <-endpointsc:
			elb.update(endpoints)
		case <-elb.quit:
			close(done)
			return
		}
	}
}

func (elb *ELB) reconcile() {
	for {
		select {
		case <-time.After(time.Second * 10):
			log.Println("running reconcile loop")
			endpoints, err := getEndpoints(elb.namespace, elb.service)
			if err != nil {
				log.Println(err)
			}
			elb.update(endpoints)
		case <-elb.quit:
			return
		}
	}
}

// Stop terminates the endpoints watcher.
func (elb *ELB) Stop() {
	close(elb.quit)
}
