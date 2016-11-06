# Kubernetes Endpoint Load Balancer

## Example Usage

```Go
package main

import (
	"log"
	"net/http"

	"github.com/kelseyhightower/endpoints"
)

func main() {
	config := &endpoints.Config{
		Namespace: "default",
		Service:   "nginx",
	}

	backends, err := endpoints.New(config)
	if err != nil {
		log.Fatal(err)
	}

	for {
		endpoint, err := backends.Next()
		if err != nil {
			log.Println(err)
			continue
		}
		_, err := http.Get(fmt.Sprintf("http://%s:%s", endpoint.Host, endpoint.Port))
		if err != nil {
			log.Println(err)
		}
	}
}
```
