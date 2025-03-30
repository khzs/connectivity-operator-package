/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConnectivitySpec defines the desired state of Connectivity.
type ConnectivitySpec struct {
	Endpoints []Endpoint `json:"endpoints"`
}

type Endpoint struct {
	URL      string   `json:"url"`
	Protocol Protocol `json:"protocol"`
}

// +kubebuilder:validation:Enum=http;grpc;ping
type Protocol string

const (
	ProtocolHTTP Protocol = "http"
	ProtocolGRPC Protocol = "grpc"
	ProtocolPing Protocol = "ping"
)

type EndpointStatus struct {
	URL                string      `json:"url"`
	Available          bool        `json:"available"`
	LastChecked        metav1.Time `json:"lastChecked"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// ConnectivityStatus defines the observed state of Connectivity.
type ConnectivityStatus struct {
	EndpointStatuses []EndpointStatus `json:"endpointStatuses,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Connectivity is the Schema for the connectivities API.
type Connectivity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConnectivitySpec   `json:"spec,omitempty"`
	Status ConnectivityStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConnectivityList contains a list of Connectivity.
type ConnectivityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Connectivity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Connectivity{}, &ConnectivityList{})
}
