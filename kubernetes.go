// Copyright 2016 Google Inc. All Rights Reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package endpoints

type object struct {
	Object endpoints `json:"object"`
	Type   string    `json:"type"`
}

type endpoints struct {
	Kind       string   `json:"kind"`
	ApiVersion string   `json:"apiVersion"`
	Metadata   metadata `json:"metadata"`
	Subsets    []subset `json:"subsets"`
	Message    string   `json:"message"`
}

type metadata struct {
	Name string `json:"name"`
}

type subset struct {
	Addresses []address `json:"addresses"`
	Ports     []port    `json:"ports"`
}

type address struct {
	IP string `json:"ip"`
}

type port struct {
	Name string `json:"name"`
	Port int32  `json:"port"`
}
