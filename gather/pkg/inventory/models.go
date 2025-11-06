// Note: This is an internal representation for gathering, before posting.
type DiscoveredDevice struct {
	DeviceType   string
	Manufacturer string
	PartNumber   string
	SerialNumber string
	ParentID     string
	RedfishURI   string
}