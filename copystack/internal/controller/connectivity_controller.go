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

package controller

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	pingpb "github.com/khzs/connectivity-listener/proto/ping"
	monitoringv1 "github.com/khzs/k8s-connectivity-operator/api/v1"
)

// ConnectivityReconciler reconciles a Connectivity object
type ConnectivityReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=monitoring.my.domain,resources=connectivities,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.my.domain,resources=connectivities/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.my.domain,resources=connectivities/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch

func (r *ConnectivityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling Connectivity", "request", req.NamespacedName)

	// Fetch the Connectivity instance
	var connectivity monitoringv1.Connectivity
	if err := r.Get(ctx, req.NamespacedName, &connectivity); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted
			return ctrl.Result{}, nil
		}
		// Error reading the object
		log.Error(err, "Failed to get Connectivity")
		return ctrl.Result{}, err
	}

	// Initialize status if it's nil
	if connectivity.Status.EndpointStatuses == nil {
		connectivity.Status.EndpointStatuses = []monitoringv1.EndpointStatus{}
	}

	// Check each endpoint
	for _, endpoint := range connectivity.Spec.Endpoints {
		endpointStatus := r.findOrCreateEndpointStatus(&connectivity, endpoint)

		// Check availability based on protocol
		available := false
		var err error

		switch endpoint.Protocol {
		case monitoringv1.ProtocolHTTP:
			available, err = r.checkHTTPEndpoint(endpoint.URL)
		case monitoringv1.ProtocolGRPC:
			available, err = r.checkGRPCEndpoint(endpoint.URL)
		case monitoringv1.ProtocolPing:
			available, err = r.checkPingEndpoint(endpoint.URL)
		}

		if err != nil {
			log.Error(err, "Failed to check endpoint", "endpoint", endpoint.URL, "protocol", endpoint.Protocol)
		}

		// Update the status
		now := metav1.Now()
		endpointStatus.LastChecked = now

		// Check if the status has changed
		if endpointStatus.Available != available {
			endpointStatus.Available = available
			endpointStatus.LastTransitionTime = now
		}
	}

	// Update the status
	if err := r.Status().Update(ctx, &connectivity); err != nil {
		log.Error(err, "Failed to update Connectivity status")
		return ctrl.Result{}, err
	}

	// Requeue after 1 minute
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

// findOrCreateEndpointStatus finds an existing status for an endpoint or creates a new one
func (r *ConnectivityReconciler) findOrCreateEndpointStatus(connectivity *monitoringv1.Connectivity, endpoint monitoringv1.Endpoint) *monitoringv1.EndpointStatus {
	for i := range connectivity.Status.EndpointStatuses {
		if connectivity.Status.EndpointStatuses[i].URL == endpoint.URL {
			return &connectivity.Status.EndpointStatuses[i]
		}
	}

	// Not found, create a new one
	newStatus := monitoringv1.EndpointStatus{
		URL:       endpoint.URL,
		Available: false,
	}
	connectivity.Status.EndpointStatuses = append(connectivity.Status.EndpointStatuses, newStatus)
	return &connectivity.Status.EndpointStatuses[len(connectivity.Status.EndpointStatuses)-1]
}

// checkHTTPEndpoint checks if an HTTP endpoint is available
func (r *ConnectivityReconciler) checkHTTPEndpoint(url string) (bool, error) {
	// Make sure the URL has a scheme
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "http://" + url
	}

	// Ensure path includes /ping
	if !strings.Contains(url, "/ping") {
		url = strings.TrimSuffix(url, "/") + "/ping"
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create request
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	// Add future timestamp header
	futureTime := time.Now().AddDate(1, 0, 0) // 1 year in the future
	futureTimeBytes, _ := futureTime.MarshalText()
	req.Header.Set("X-Timestamp", string(futureTimeBytes))

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check if response is 200 OK
	return resp.StatusCode == http.StatusOK, nil
}

// checkGRPCEndpoint checks if a gRPC endpoint is available
func (r *ConnectivityReconciler) checkGRPCEndpoint(url string) (bool, error) {
	// Connect to the gRPC server
	conn, err := grpc.Dial(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return false, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}
	defer conn.Close()

	// Create a client
	client := pingpb.NewPingServiceClient(conn)

	// Generate two random numbers
	a := rand.IntN(100)
	b := rand.IntN(100)

	// Calculate expected hash
	sum := a + b
	expectedHash := sha1.Sum([]byte(strconv.Itoa(sum)))
	expectedHashStr := hex.EncodeToString(expectedHash[:])

	// Make the request
	resp, err := client.Ping(context.Background(), &pingpb.PingRequest{
		A: int32(a),
		B: int32(b),
	})
	if err != nil {
		return false, fmt.Errorf("gRPC Ping failed: %w", err)
	}

	// Verify the hash
	return resp.Hash == expectedHashStr, nil
}

// checkPingEndpoint checks if an ICMP ping endpoint is available
func (r *ConnectivityReconciler) checkPingEndpoint(host string) (bool, error) {
	// Create a "udp" connection because ip4:icmp drops a privilege error
	c, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return false, fmt.Errorf("failed to listen for ICMP packets: %w", err)
	}
	defer c.Close()

	// Create ICMP echo request
	wm := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: os.Getpid() & 0xffff, Seq: 1,
			Data: []byte("PING"),
		},
	}

	wb, err := wm.Marshal(nil)
	if err != nil {
		return false, fmt.Errorf("failed to marshal ICMP message: %w", err)
	}

	// Resolve host to IP address
	ips, err := net.LookupIP(host)
	if err != nil {
		return false, fmt.Errorf("could not resolve host %s: %w", host, err)
	}

	// Use the first IP
	if len(ips) == 0 {
		return false, fmt.Errorf("no IP addresses found for host %s", host)
	}

	ipv4Addr := ips[0].To4()
	if ipv4Addr == nil {
		return false, fmt.Errorf("no IPv4 address found for host %s", host)
	}

	// Set deadline for reading
	if err := c.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return false, fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Send the request
	_, err = c.WriteTo(wb, &net.UDPAddr{IP: ipv4Addr, Port: 0})
	if err != nil {
		return false, fmt.Errorf("failed to send ICMP packet: %w", err)
	}

	// Wait for reply
	rb := make([]byte, 1500)
	n, _, err := c.ReadFrom(rb)
	if err != nil {
		return false, fmt.Errorf("failed to receive ICMP packet: %w", err)
	}

	// Parse the reply
	rm, err := icmp.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), rb[:n])
	if err != nil {
		return false, fmt.Errorf("failed to parse ICMP message: %w", err)
	}

	// Check if it's an echo reply
	switch rm.Type {
	case ipv4.ICMPTypeEchoReply:
		return true, nil
	default:
		return false, fmt.Errorf("received unexpected ICMP message type: %v", rm.Type)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConnectivityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1.Connectivity{}).
		Named("connectivity").
		Complete(r)
}
