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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

var (
	apiHost            = "127.0.0.1:8001"
	endpointsPath      = "/api/v1/namespaces/%s/endpoints/%s"
	endpointsWatchPath = "/api/v1/watch/namespaces/%s/endpoints/%s"
)

type Object struct {
	Object Endpoints `json:"object"`
	Type   string    `json:"type"`
}

type Endpoints struct {
	Kind       string   `json:"kind"`
	ApiVersion string   `json:"apiVersion"`
	Metadata   Metadata `json:"metadata"`
	Subsets    []Subset `json:"subsets"`
	Message    string   `json:"message"`
}

type Metadata struct {
	Name string `json:"name"`
}

type Subset struct {
	Addresses []Address `json:"addresses"`
	Ports     []Port    `json:"ports"`
}

type Address struct {
	IP string `json:"ip"`
}

type Port struct {
	Name string `json:"name"`
	Port int32  `json:"port"`
}

// Endpoint holds a Kubernetes endpoint.
type Endpoint struct {
	Host  string
	Port  string
	Ports map[string]string
}

var ErrNotExist = errors.New("does not exist")

func getEndpoints(namespace, service string) ([]Endpoint, error) {
	path := fmt.Sprintf(endpointsPath, namespace, service)
	request := &http.Request{
		Header: make(http.Header),
		Method: http.MethodGet,
		URL: &url.URL{
			Host:   apiHost,
			Path:   path,
			Scheme: "http",
		},
	}

	var eps Endpoints
	r, err := doRequest(context.TODO(), request)
	if err != nil {
		return nil, err
	}

	err = json.NewDecoder(r).Decode(&eps)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return formatEndpoints(eps), nil
}

func watchEndpoints(namespace, service string, done <-chan struct{}) (<-chan []Endpoint, <-chan error) {
	endpointsc := make(chan []Endpoint)
	errc := make(chan error, 1)

	path := fmt.Sprintf(endpointsWatchPath, namespace, service)
	request := &http.Request{
		Header: make(http.Header),
		Method: http.MethodGet,
		URL: &url.URL{
			Host:   apiHost,
			Path:   path,
			Scheme: "http",
		},
	}
	request.Header.Set("Accept", "application/json, */*")

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			for {
				r := request.WithContext(ctx)
				resp, err := http.DefaultClient.Do(r)

				if ctx.Err() == context.Canceled {
					break
				}
				if err != nil {
					errc <- err
					time.Sleep(5 * time.Second)
					resp.Body.Close()
					continue
				}
				if resp.StatusCode == 404 {
					errc <- ErrNotExist
					time.Sleep(5 * time.Second)
					resp.Body.Close()
					continue
				}
				if resp.StatusCode != 200 {
					errc <- errors.New("error non 200 reponse: " + resp.Status)
					time.Sleep(5 * time.Second)
					resp.Body.Close()
					continue
				}

				decoder := json.NewDecoder(resp.Body)
				for {
					var o Object
					err := decoder.Decode(&o)
					if ctx.Err() == context.Canceled {
						resp.Body.Close()
						break
					}
					if err != nil {
						errc <- err
						resp.Body.Close()
						break
					}
					if o.Type == "ERROR" {
						errc <- errors.New(o.Object.Message)
						resp.Body.Close()
						break
					}
					endpointsc <- formatEndpoints(o.Object)
				}
			}
			wg.Done()
		}()

		select {
		case <-done:
			cancel()
			wg.Wait()
			close(endpointsc)
			close(errc)
		}
	}()

	return endpointsc, errc
}

func doRequest(ctx context.Context, request *http.Request) (io.ReadCloser, error) {
	request.Header.Set("Accept", "application/json, */*")

	r := request.WithContext(ctx)
	resp, err := http.DefaultClient.Do(r)
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
