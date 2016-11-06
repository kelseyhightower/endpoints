// Copyright 2016 Google Inc. All Rights Reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

/*
Package endpoints provides an implementation of a client-side
load balancer that distributes client request across a collection
of Kubernetes endpoints using a basic Round-robin algorithm.

The endpoints load balancer does not operate in the data path and
only manages a list of backends based on services defined in a
Kubernetes cluster.

Load balancers are created using a Config:

    config := &endpoints.Config{
        Namespace: namespace,
        Service:   service,
    }
    lb := endpoints.New(config)

Before an endpoints load balancer can be used the endpoints must
be synchronized from the Kubernetes API:

    err := lb.SyncEndpoints()

Once the initial set of endpoints have been populated clients
can call the Next method to the next endpoint:

    endpoint, err := lb.Next()
    // ...
    url := fmt.Sprintf("http://%s:%s", endpoint.Host, endpoint.Port)
    resp, err := c.Get(url)
    // ...

Clients can synchronize the endpoints load balancer backends in the
background:

    err := lb.StartBackgroundSync() 

Shutdown terminates a load balancer instance by stopping any
background watches and reconciliation loops against the Kubernetes
API server:

    lb.Shutdown()
*/
package endpoints
