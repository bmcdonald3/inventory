// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package discoverysnapshot

import (
	"context"
	"encoding/json"

	"github.com/openchami/fabrica/pkg/resource"
)

// DiscoverySnapshot is the resource that holds a raw hardware snapshot.
type DiscoverySnapshot struct {
	resource.Resource `json:",inline"`
	Spec              DiscoverySnapshotSpec   `json:"spec" validate:"required"`
	Status            DiscoverySnapshotStatus `json:"status,omitempty"`
}

// DiscoverySnapshotSpec defines the desired state of DiscoverySnapshot
type DiscoverySnapshotSpec struct {
	// RawData holds the complete, raw JSON payload from a discovery tool (e.g., the collector).
	// The reconciler will parse this.
	RawData json.RawMessage `json:"rawData" validate:"required"`
}

// DiscoverySnapshotStatus defines the observed state of DiscoverySnapshot
type DiscoverySnapshotStatus struct {
	Phase   string   `json:"phase,omitempty"`   // e.g., Pending, Processing, Complete, Error
	Message string   `json:"message,omitempty"` // A human-readable message
	Logs    []string `json:"logs,omitempty"`    // Logs generated during reconciliation
}

// Validate is a hook for custom validation logic
func (r *DiscoverySnapshot) Validate(ctx context.Context) error {
	// You can add logic here to ensure RawData is valid JSON, etc.
	return nil
}

func init() {
	// Register the resource with Fabrica
	resource.RegisterResourcePrefix("DiscoverySnapshot", "ds")
}