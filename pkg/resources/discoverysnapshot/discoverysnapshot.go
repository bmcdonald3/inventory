// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package discoverysnapshot

import (
	"context"
	"encoding/json" // Make sure this import is here
	"github.com/openchami/fabrica/pkg/resource"
)

// DiscoverySnapshot represents a DiscoverySnapshot resource
type DiscoverySnapshot struct {
	resource.Resource
	Spec   DiscoverySnapshotSpec   `json:"spec" validate:"required"`
	Status DiscoverySnapshotStatus `json:"status,omitempty"`
}

// DiscoverySnapshotSpec defines the desired state of DiscoverySnapshot
type DiscoverySnapshotSpec struct {
	// Example: "redfish-collector"
	Source string `json:"source,omitempty"`

	// Target is the identifier for the collection target.
	// Example: "172.24.0.2"
	Target string `json:"target,omitempty"`

	// RawData holds the complete, raw JSON/YAML payload from the collector.
	RawData json.RawMessage `json:"rawData,omitempty"`
}

// DiscoverySnapshotStatus defines the observed state of DiscoverySnapshot
type DiscoverySnapshotStatus struct {
	// Phase indicates the current state of reconciliation.
	// Examples: "Pending", "Processing", "Complete", "Error"
	Phase string `json:"phase,omitempty"`

	// Message provides a human-readable summary of the snapshot's status.
	Message string `json:"message,omitempty"`

	// Logs stores a list of log entries or errors generated during processing.
	Logs []string `json:"logs,omitempty"`

	// DevicesCreated holds the UIDs of all Device resources created by this snapshot.
	DevicesCreated []string `json:"devicesCreated,omitempty"`

	// DevicesUpdated holds the UIDs of all Device resources updated by this snapshot.
	DevicesUpdated []string `json:"devicesUpdated,omitempty"`

	// Conditions store the history of transient conditions across phases
	Conditions []resource.Condition `json:"conditions,omitempty"`
}

// Validate implements custom validation logic for DiscoverySnapshot
func (r *DiscoverySnapshot) Validate(ctx context.Context) error {
	// Add custom validation logic here
	return nil
}

// GetKind returns the kind of the resource
func (r *DiscoverySnapshot) GetKind() string {
	return "DiscoverySnapshot"
}

// GetName returns the name of the resource
func (r *DiscoverySnapshot) GetName() string {
	return r.Metadata.Name
}

// GetUID returns the UID of the resource
func (r *DiscoverySnapshot) GetUID() string {
	return r.Metadata.UID
}

func init() {
	// Register resource type prefix for storage
	resource.RegisterResourcePrefix("DiscoverySnapshot", "dis")
}
