# Kubernetes Endpoint Load Balancer

## Example Usage

```Go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/kelseyhightower/endpoints"
)

func main() {
	backends, err := endpoints.New(&endpoints.Config{
		Namespace: "default",
		Service:   "nginx",
	})
	if err != nil {
		log.Fatal(err)
	}

	endpoint, err := backends.Next()
	if err != nil {
		log.Fatal(err)
	}

	urlStr := fmt.Sprintf("http://%s:%s", endpoint.Host, endpoint.Port)
	resp, err := http.Get(urlStr)
	if err != nil {
		log.Println(err)
		continue
	}
	resp.Body.Close()
	log.Printf("Endpoint %s response code: %s", urlStr, resp.Status)
}
```
