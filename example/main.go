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

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kelseyhightower/elb"
)

func main() {
	lb, err := elb.New("", "nginx")
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			endpoint, err := lb.Next()
			if err != nil {
				log.Println(err)
				time.Sleep(500 * time.Millisecond)
				continue
			}
			urlStr := fmt.Sprintf("http://%s:%s", endpoint.Host, endpoint.Port)
			resp, err := http.Get(urlStr)
			if err != nil {
				log.Println(err)
				continue
			}
			resp.Body.Close()
			log.Printf("Endpoint %s response code: %s", urlStr, resp.Status)
			time.Sleep(100 * time.Millisecond)
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-signalChan:
			log.Printf("Shutdown signal received, exiting...")
			lb.Stop()
			time.Sleep(5 * time.Second)
			os.Exit(0)
		}
	}
}
