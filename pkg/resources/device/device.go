/*
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
*/

package device

import (
	"encoding/json"
	"time"

	"github.com/openchami/fabrica/pkg/resource" // Use this import
)

// Device is the envelope for the Device resource.
// This struct now embeds resource.Resource, just like your FRU example.
type Device struct {
	resource.Resource `json:",inline"`
	Spec              DeviceSpec   `json:"spec"`
	Status            DeviceStatus `json:"status"`
}

// DeviceSpec defines the desired state of a Device.
// We are keeping this empty, as you originally requested.
type DeviceSpec struct {
	// All fields are in Status.
}

type DeviceStatus struct {
	DeviceType   string `json:"deviceType,omitempty" validate:"omitempty,oneof=Node GPU Rack"`
	Manufacturer string `json:"manufacturer,omitempty"`
	PartNumber   string `json:"partNumber,omitempty"`
	SerialNumber string `json:"serialNumber,omitempty"`

	// 'parentID' refers to the metadata.uid of the parent Device
	ParentID string `json:"parentID,omitempty" validate:"omitempty,uuid4"`

	// 'childrenDeviceIds' is a read-only list of metadata.uids
	ChildrenDeviceIds []string `json:"childrenDeviceIds,omitempty" validate:"omitempty,dive,uuid4"`

	// Arbitrary key-value store for custom attributes
	Properties map[string]json.RawMessage `json:"properties,omitempty"`

	// Metadata fields from your model not in the standard envelope
	SchemaVersion string `json:"schemaVersion,omitempty"`

	// Timestamp for soft deletes
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// GetKind returns the kind of the resource
func (d *Device) GetKind() string {
	return "Device"
}

// GetName returns the name of the resource
func (d *Device) GetName() string {
	return d.Metadata.Name
}

// GetUID returns the UID of the resource
func (d *Device) GetUID() string {
	return d.Metadata.UID
}

func init() {
	// Register resource type prefix for storage
	resource.RegisterResourcePrefix("Device", "dev")
}