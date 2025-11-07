package inventory

// DiscoveredDevice is the internal representation of a device before posting to the API.
type DiscoveredDevice struct {
	DeviceType   string
	Manufacturer string
	PartNumber   string
	SerialNumber string
	ParentID     string // Redfish URI placeholder for the parent
	RedfishURI   string // Source URI
}

// SystemInventory holds the discovered devices related to one System/Node.
type SystemInventory struct {
	Node  DiscoveredDevice
	CPUs  []DiscoveredDevice
	DIMMs []DiscoveredDevice
}

// RedfishCollection defines the structure for Redfish collection responses (e.g., /Systems).
type RedfishCollection struct {
	Members []struct {
		ODataID string `json:"@odata.id"`
	} `json:"Members"`
}

// CommonRedfishProperties contains the fields required by the Device model,
// found in various Redfish resources (System, Processor, Memory).
type CommonRedfishProperties struct {
	Manufacturer string `json:"Manufacturer,omitempty"`
	Model        string `json:"Model,omitempty"`
	PartNumber   string `json:"PartNumber,omitempty"`
	SerialNumber string `json:"SerialNumber,omitempty"`
}

// RedfishSystem defines the structure for a System resource (the Node).
type RedfishSystem struct {
	CommonRedfishProperties // Embeds the common fields
	Processors struct {
		ODataID string `json:"@odata.id"`
	} `json:"Processors"`
	Memory struct {
		ODataID string `json:"@odata.id"`
	} `json:"Memory"`
}

// RedfishProcessor defines the structure for a Processor resource (the CPU).
type RedfishProcessor struct {
	CommonRedfishProperties // Embeds the common fields
}

// RedfishMemory defines the structure for a Memory resource (the DIMM).
type RedfishMemory struct {
	CommonRedfishProperties // Embeds the common fields
}

// Structs for API interaction
type Metadata struct {
	UID string `json:"uid"`
}

type DeviceResponse struct {
	Metadata Metadata `json:"metadata"`
}