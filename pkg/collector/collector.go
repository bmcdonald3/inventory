package collector

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	// Import the Fabrica-generated client (the SDK)
	fabricaclient "github.com/user/inventory-api/pkg/client"
	// Import the API's canonical resource definition
	"github.com/user/inventory-api/pkg/resources/device"
)

// --- Configuration ---

// InventoryAPIHost is the address of the Fabrica API server.
const InventoryAPIHost = "http://localhost:8081"

// DefaultUsername and DefaultPassword are hardcoded for Redfish basic auth.
const DefaultUsername = "root"
const DefaultPassword = "initial0" // Updated to your password

// --- Main Orchestration Function ---

// CollectAndPost is the main function for the collector.
// It connects to a BMC, discovers hardware, and posts it to the API.
func CollectAndPost(bmcIP string) error {
	// 1. Initialize Redfish Client
	rfClient, err := NewRedfishClient(bmcIP, DefaultUsername, DefaultPassword)
	if err != nil {
		return fmt.Errorf("failed to initialize Redfish client: %w", err)
	}

	fmt.Println("Starting Redfish discovery...")

	// --- 2. REDFISH DISCOVERY (Live Call) ---
	// This function now returns the API's *device.DeviceStatus structs
	// and a map to track parent/child relationships.
	deviceStatuses, childToParentURI, err := discoverDevices(rfClient)
	if err != nil {
		return fmt.Errorf("redfish discovery failed: %w", err)
	}
	if len(deviceStatuses) == 0 {
		return errors.New("redfish discovery found no devices to post")
	}
	fmt.Printf("Redfish Discovery Complete: Found %d total devices.\n", len(deviceStatuses))

	// --- 3. INITIALIZE API CLIENT (THE SDK) ---
	sdkClient, err := fabricaclient.NewClient(InventoryAPIHost, nil)
	if err != nil {
		return fmt.Errorf("failed to create fabrica client: %w", err)
	}

	// Use a context for API calls
	ctx := context.Background()

	// This map translates Redfish URIs to API UUIDs
	uriToUID := make(map[string]string)

	// --- 4. API POSTING (Using the SDK) ---

	// 1. Post Parent Devices (Node)
	for _, status := range deviceStatuses {
		// We stored the redfish_uri in the properties map during discovery
		redfishURI := status.Properties["redfish_uri"].(string)

		// Check if it's a parent (it will not be in the child map)
		if _, isChild := childToParentURI[redfishURI]; !isChild {
			tempName := fmt.Sprintf("%s-%s", status.DeviceType, status.SerialNumber)

			fmt.Printf("-> Creating resource envelope for (Parent) %s...\n", tempName)

			// Use SDK Create method
			createReq := fabricaclient.CreateDeviceRequest{Name: tempName}
			createdDevice, err := sdkClient.CreateDevice(ctx, createReq)
			if err != nil {
				return fmt.Errorf("SDK Create failed for %s: %w", tempName, err)
			}

			uid := createdDevice.Metadata.UID
			uriToUID[redfishURI] = uid // Store for children

			fmt.Printf("-> Updating status for %s (UID: %s)...\n", tempName, uid)

			// Use our NEW manual UpdateDeviceStatus method (from File 2)
			_, err = sdkClient.UpdateDeviceStatus(ctx, uid, status)
			if err != nil {
				return fmt.Errorf("SDK UpdateStatus failed for %s: %w", tempName, err)
			}
			fmt.Printf("-> Successfully posted parent device %s\n", uid)
		}
	}

	// 2. Post Child Devices (CPU, DIMM)
	for _, status := range deviceStatuses {
		redfishURI := status.Properties["redfish_uri"].(string)

		// Check if it's a child
		if parentURI, isChild := childToParentURI[redfishURI]; isChild {
			// Find the parent's *API UUID*
			parentUUID, ok := uriToUID[parentURI]
			if !ok {
				fmt.Printf("Warning: Failed to find parent UUID for %s. Skipping.\n", parentURI)
				continue
			}

			// Add the resolved ParentID to the status struct
			status.ParentID = parentUUID

			tempName := fmt.Sprintf("%s-%s", status.DeviceType, status.SerialNumber)

			fmt.Printf("-> Creating resource envelope for (Child) %s...\n", tempName)

			// Use SDK Create method
			createReq := fabricaclient.CreateDeviceRequest{Name: tempName}
			createdDevice, err := sdkClient.CreateDevice(ctx, createReq)
			if err != nil {
				return fmt.Errorf("SDK Create failed for %s: %w", tempName, err)
			}

			uid := createdDevice.Metadata.UID

			fmt.Printf("-> Updating status for %s (UID: %s)...\n", tempName, uid)

			// Use our NEW manual UpdateDeviceStatus method (from File 2)
			_, err = sdkClient.UpdateDeviceStatus(ctx, uid, status)
			if err != nil {
				return fmt.Errorf("SDK UpdateStatus failed for %s: %w", tempName, err)
			}
			fmt.Printf("-> Successfully posted child device %s\n", uid)
		}
	}

	return nil
}

// --- Redfish Client Struct and Methods ---

// RedfishClient holds connection details and the HTTP client instance.
type RedfishClient struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// NewRedfishClient initializes the client with a specified BMC IP.
func NewRedfishClient(bmcIP, username, password string) (*RedfishClient, error) {
	baseURL := fmt.Sprintf("https://%s/redfish/v1", bmcIP)

	// Create a custom HTTP client that trusts the BMC's self-signed certificate.
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &RedfishClient{
		BaseURL:    baseURL,
		Username:   username,
		Password:   password,
		HTTPClient: &http.Client{Transport: tr},
	}, nil
}

// Get makes an authenticated GET request to a Redfish path.
func (c *RedfishClient) Get(path string) ([]byte, error) {
	targetURL, err := url.JoinPath(c.BaseURL, path)
	if err != nil {
		return nil, fmt.Errorf("failed to join path: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redfish request for %s: %w", targetURL, err)
	}

	req.SetBasicAuth(c.Username, c.Password)
	req.Header.Add("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Redfish request for %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Redfish API returned status code %d for %s", resp.StatusCode, targetURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

// --- Redfish Discovery and Mapping Functions ---

// discoverDevices uses the Redfish client to walk the resource hierarchy.
// It returns the list of mapped status structs and a map to track parent/child URIs.
func discoverDevices(c *RedfishClient) ([]*device.DeviceStatus, map[string]string, error) {
	var statuses []*device.DeviceStatus
	// childToParentURI maps a child's RedfishURI to its parent's RedfishURI
	childToParentURI := make(map[string]string)

	// --- 1. Get Systems Collection ---
	systemsBody, err := c.Get("/Systems")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get Systems collection: %w", err)
	}

	var systemsCollection RedfishCollection
	if err := json.Unmarshal(systemsBody, &systemsCollection); err != nil {
		return nil, nil, fmt.Errorf("failed to decode Systems collection: %w", err)
	}

	// --- 2. Iterate through each System (Node) ---
	for _, member := range systemsCollection.Members {
		systemURI := strings.TrimPrefix(member.ODataID, "/redfish/v1")

		systemInventory, err := getSystemInventory(c, systemURI)
		if err != nil {
			fmt.Printf("Warning: Failed to get inventory for system %s: %v\n", member.ODataID, err)
			continue
		}

		// Add the Node's status
		statuses = append(statuses, systemInventory.NodeStatus)

		// Add CPU statuses and map their parents
		for _, cpuStatus := range systemInventory.CPUs {
			// Get the Redfish URI we stored in properties
			cpuRedfishURI := cpuStatus.Properties["redfish_uri"].(string)
			statuses = append(statuses, cpuStatus)
			childToParentURI[cpuRedfishURI] = systemURI
		}

		// Add DIMM statuses and map their parents
		for _, dimmStatus := range systemInventory.DIMMs {
			// Get the Redfish URI we stored in properties
			dimmRedfishURI := dimmStatus.Properties["redfish_uri"].(string)
			statuses = append(statuses, dimmStatus)
			childToParentURI[dimmRedfishURI] = systemURI
		}
	}

	return statuses, childToParentURI, nil
}

// getSystemInventory discovers a single system (Node) and its children.
func getSystemInventory(c *RedfishClient, systemURI string) (*SystemInventory, error) {
	inv := &SystemInventory{CPUs: make([]*device.DeviceStatus, 0), DIMMs: make([]*device.DeviceStatus, 0)}

	// --- 1. Get and Map System (Node) Details ---
	systemBody, err := c.Get(systemURI)
	if err != nil {
		return nil, err
	}

	var systemData RedfishSystem
	if err := json.Unmarshal(systemBody, &systemData); err != nil {
		return nil, fmt.Errorf("failed to decode system data from %s: %w", systemURI, err)
	}

	// Map Node Data
	inv.NodeStatus = mapCommonProperties(
		systemData.CommonRedfishProperties,
		"Node",
		systemURI,
	)

	// --- 2. Get Processors (CPUs) ---
	if cpuCollectionURI := systemData.Processors.ODataID; cpuCollectionURI != "" {
		cleanedURI := strings.TrimPrefix(cpuCollectionURI, "/redfish/v1")
		cpuDevices, err := getCollectionDevices(c, cleanedURI, "CPU", &RedfishProcessor{})
		if err != nil {
			fmt.Printf("Warning: Failed to retrieve CPU inventory from %s: %v\n", cpuCollectionURI, err)
		} else {
			inv.CPUs = cpuDevices
		}
	}

	// --- 3. Get Memory (DIMMs) ---
	if dimmCollectionURI := systemData.Memory.ODataID; dimmCollectionURI != "" {
		cleanedURI := strings.TrimPrefix(dimmCollectionURI, "/redfish/v1")
		dimmDevices, err := getCollectionDevices(c, cleanedURI, "DIMM", &RedfishMemory{})
		if err != nil {
			fmt.Printf("Warning: Failed to retrieve DIMM inventory from %s: %v\n", dimmCollectionURI, err)
		} else {
			inv.DIMMs = dimmDevices
		}
	}

	return inv, nil
}

// getCollectionDevices retrieves a collection, iterates over members, and maps them.
func getCollectionDevices(c *RedfishClient, collectionURI, deviceType string, componentTypeExample interface{}) ([]*device.DeviceStatus, error) {
	var statuses []*device.DeviceStatus

	collectionBody, err := c.Get(collectionURI)
	if err != nil {
		return nil, err
	}

	var collection RedfishCollection
	if err := json.Unmarshal(collectionBody, &collection); err != nil {
		return nil, fmt.Errorf("failed to decode collection from %s: %w", collectionURI, err)
	}

	for _, member := range collection.Members {
		memberURI := strings.TrimPrefix(member.ODataID, "/redfish/v1")
		memberBody, err := c.Get(memberURI)
		if err != nil {
			fmt.Printf("Warning: Failed to get member %s: %v\n", member.ODataID, err)
			continue
		}

		// Use reflection to unmarshal into the correct component type
		component := reflect.New(reflect.TypeOf(componentTypeExample).Elem()).Interface()
		if err := json.Unmarshal(memberBody, &component); err != nil {
			fmt.Printf("Warning: Failed to unmarshal component %s: %v\n", member.ODataID, err)
			continue
		}

		// Use reflection to access the embedded CommonRedfishProperties
		rfProps := reflect.ValueOf(component).Elem().Field(0).Interface().(CommonRedfishProperties)

		// Append the new DeviceStatus struct
		statuses = append(statuses, mapCommonProperties(rfProps, deviceType, memberURI))
	}

	return statuses, nil
}

// mapCommonProperties maps Redfish fields to the API's DeviceStatus struct.
func mapCommonProperties(rfProps CommonRedfishProperties, deviceType, redfishURI string) *device.DeviceStatus {
	partNum := rfProps.PartNumber
	if partNum == "" {
		partNum = rfProps.Model
	}

	// Map directly to the API's DeviceStatus struct
	return &device.DeviceStatus{
		DeviceType:   deviceType,
		Manufacturer: rfProps.Manufacturer,
		PartNumber:   partNum,
		SerialNumber: rfProps.SerialNumber,
		// ParentID is set later in the main posting loop
		Properties: map[string]interface{}{
			"redfish_uri": redfishURI, // Store this for parent mapping
		},
	}
}