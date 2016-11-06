# Endpoints Load Balancer

[![GoDoc](https://godoc.org/github.com/kelseyhightower/endpoints?status.svg)](http://godoc.org/github.com/kelseyhightower/endpoints)

## Overview

Package endpoints provides an implementation of a client-side load balancer that distributes client request across a collection of Kubernetes endpoints using a basic Round-robin algorithm. The endpoints load balancer does not operate in the data path and only manages a list of backends based on services defined in a Kubernetes cluster.

<img src="https://github.com/kelseyhightower/endpoints/blob/master/endpoints.png" width="500">

### Use Cases

* Reduce latency by avoiding DNS lookups. Endpoints are returned as IP addresses and one or more ports.
* Reduce latency by avoiding extra hops. Client side load balancing enables direct pod to pod communication.

### Drawbacks

* Increased load on the Kubernetes API server. Each client watches for changes and runs a reconciliation loop against the Kubernetes API.
* External dependency. Kubernetes service discovery works out of the box; using this library adds a new dependency.

## Example Usage

See the example application for usage details.
