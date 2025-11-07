package inventory

import (
	"fmt"
	"bytes"
	"encoding/json"
	"net/http"
	"crypto/tls"
	"io"
	"errors"
	"reflect"
	"net/url"
	"strings"

	"github.com/bmcdonald3/inventory/pkg/resources/device"
)

// Placeholder for the actual API server address
const InventoryAPIHost = "http://localhost:8081"
const DefaultUsername = "root"
const DefaultPassword = "initial0" // Security note: Hardcoding credentials is not recommended in production.

// discoverDevices uses the Redfish client to walk the resource hierarchy
// and extract device information.
func discoverDevices(c *RedfishClient) ([]DiscoveredDevice, error) {
	var devices []DiscoveredDevice
    
    // --- 1. Get Systems Collection ---
	systemsBody, err := c.Get("/Systems")
	if err != nil {
		return nil, fmt.Errorf("failed to get Systems collection: %w", err)
	}

	var systemsCollection RedfishCollection
	if err := json.Unmarshal(systemsBody, &systemsCollection); err != nil {
		return nil, fmt.Errorf("failed to decode Systems collection: %w", err)
	}

	// --- 2. Iterate through each System (Node) ---
	for _, member := range systemsCollection.Members {
		systemURI := strings.TrimPrefix(member.ODataID, "/redfish/v1")
		// Traverse into the System resource (the Node)
		systemInventory, err := getSystemInventory(c, systemURI)
		if err != nil {
			// Log the error and continue to the next system
			fmt.Printf("Warning: Failed to get inventory for system %s: %v\n", systemURI, err)
			continue
		}
        
        // Collect Node, CPUs, and DIMMs
		devices = append(devices, systemInventory.Node)
		devices = append(devices, systemInventory.CPUs...)
		devices = append(devices, systemInventory.DIMMs...)
	}

	fmt.Printf("Redfish Discovery Complete: Found %d total devices.\n", len(devices))
	return devices, nil
}

func CollectAndPost(
	bmcIP string) error {
    // 1. Initialize Redfish Client
    rfClient, err := NewRedfishClient(bmcIP, DefaultUsername, DefaultPassword)
    if err != nil {
        return fmt.Errorf("failed to initialize Redfish client: %w", err)
    }

    // Optional debug check (can be removed)
    body, err := rfClient.Get("") 
    if err != nil {
        return fmt.Errorf("redfish client test failed: %w", err)
    }
    fmt.Printf("Redfish Client Test: Successfully connected to %s. Response size: %d bytes.\n", rfClient.BaseURL, len(body))


    // --- 2. REDFISH DISCOVERY (Live Call) ---
    // Note: We use '=' here, not ':='
    devices, err := discoverDevices(rfClient) 
    if err != nil {
        return fmt.Errorf("redfish discovery failed: %w", err)
    }
    
    if len(devices) == 0 {
        return errors.New("redfish discovery found no devices to post")
    }

    nameToUID := make(map[string]string)
    
    // Sort devices to ensure parents are posted before children.
    // We assume Node is always the parent for CPUs/DIMMs.
    // We rely on the parent ID being an empty string for the Node device.
    
    // 1. Post Parent Devices (Devices with no ParentID URI)
    for _, dev := range devices {
        if dev.ParentID == "" {
            // This is a top-level device (Node)
            tempName := fmt.Sprintf("%s-%s", dev.DeviceType, dev.SerialNumber)
            
            fmt.Printf("-> Creating resource envelope for %s (%s)...\n", tempName, dev.DeviceType)
            uid, err := createDeviceEnvelope(tempName)
            if err != nil {
                return fmt.Errorf("failed to create API envelope for %s: %w", tempName, err)
            }
            // Store the API-assigned UUID, mapped by the Redfish URI for lookup by children
            nameToUID[dev.RedfishURI] = uid 
            
            // Update status (without ParentID for the top-level device)
            statusData := map[string]interface{}{
                "deviceType":   dev.DeviceType,
                "manufacturer": dev.Manufacturer,
                "partNumber":   dev.PartNumber,
                "serialNumber": dev.SerialNumber,
                "properties": map[string]interface{}{
                    "redfish_uri": dev.RedfishURI,
                },
            }
            fmt.Printf("-> Updating status for %s (UID: %s)...\n", tempName, uid)
            if err := updateDeviceStatus(uid, statusData); err != nil {
                return fmt.Errorf("failed to update status for %s: %w", tempName, err)
            }
            fmt.Printf("-> Successfully posted parent device %s\n", uid)
        }
    }

    // 2. Post Child Devices (Devices with a ParentID URI)
    for i, dev := range devices {
        if dev.ParentID != "" {
            // This is a child device (CPU, DIMM, etc.)
            
            // Resolve Redfish URI ParentID to API UUID
            parentUUID, ok := nameToUID[dev.ParentID]
            if !ok {
                 fmt.Printf("Warning: Failed to find parent UUID for %s. Skipping.\n", dev.ParentID)
                 continue
            }

            tempName := fmt.Sprintf("%s-%s-%d", dev.DeviceType, dev.SerialNumber, i)
            
            fmt.Printf("-> Creating resource envelope for %s (%s)...\n", tempName, dev.DeviceType)
            uid, err := createDeviceEnvelope(tempName)
            if err != nil {
                return fmt.Errorf("failed to create API envelope for %s: %w", tempName, err)
            }
            
            // Update status (including the resolved ParentID UUID)
            statusData := map[string]interface{}{
                "deviceType":   dev.DeviceType,
                "manufacturer": dev.Manufacturer,
                "partNumber":   dev.PartNumber,
                "serialNumber": dev.SerialNumber,
                "parentID":     parentUUID, // <--- RESOLVED UUID HERE
                "properties": map[string]interface{}{
                    "redfish_uri": dev.RedfishURI,
                },
            }
            fmt.Printf("-> Updating status for %s (UID: %s)...\n", tempName, uid)
            if err := updateDeviceStatus(uid, statusData); err != nil {
                return fmt.Errorf("failed to update status for %s: %w", tempName, err)
            }
            fmt.Printf("-> Successfully posted child device %s\n", uid)
        }
    }
    
    return nil
}

// NewRedfishClient initializes the client with a specified BMC IP.
func NewRedfishClient(bmcIP, username, password string) (*RedfishClient, error) {
	// Redfish requires HTTPS and starts at the /redfish/v1 path.
	baseURL := fmt.Sprintf("https://%s/redfish/v1", bmcIP)

	// Create a custom HTTP client that trusts the BMC's self-signed certificate.
	// NOTE: In production, you would use proper certificate validation.
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	
	return &RedfishClient{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		HTTPClient: &http.Client{Transport: tr},
	}, nil
}

func (c *RedfishClient) Get(path string) ([]byte, error) {
	// The path can be a full URI or a relative path (e.g., /Systems/1).
	targetURL, err := url.JoinPath(c.BaseURL, path)
    if err != nil {
        return nil, fmt.Errorf("failed to join path: %w", err)
    }
	
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redfish request for %s: %w", targetURL, err)
	}

	// Add Basic Authentication header
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


func mapCommonProperties(rfProps CommonRedfishProperties, deviceType, redfishURI, parentURI string) DiscoveredDevice {
    // Redfish often uses Model if PartNumber is unavailable or vice-versa.
    // Prioritize PartNumber, fallback to Model.
    partNum := rfProps.PartNumber
    if partNum == "" {
        partNum = rfProps.Model
    }

	return DiscoveredDevice{
		DeviceType:   deviceType,
		Manufacturer: rfProps.Manufacturer,
		PartNumber:   partNum,
		SerialNumber: rfProps.SerialNumber,
		ParentID:     parentURI, // This will be resolved to a UUID later (Step 5)
		RedfishURI:   redfishURI,
	}
}

func getSystemInventory(c *RedfishClient, systemURI string) (*SystemInventory, error) {
	inv := &SystemInventory{CPUs: make([]DiscoveredDevice, 0), DIMMs: make([]DiscoveredDevice, 0)}

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
    inv.Node = mapCommonProperties(
        systemData.CommonRedfishProperties, 
        "Node", 
        systemURI, 
        "", // Parent will be resolved later (Chassis/Rack, etc.)
    )
    
    // --- 2. Get Processors (CPUs) ---
    if cpuCollectionURI := systemData.Processors.ODataID; cpuCollectionURI != "" {
		cleanedURI := strings.TrimPrefix(cpuCollectionURI, "/redfish/v1")
        cpuDevices, err := getCollectionDevices(c, cleanedURI, "CPU", systemURI, &RedfishProcessor{})
        if err != nil {
            fmt.Printf("Warning: Failed to retrieve CPU inventory from %s: %v\n", cpuCollectionURI, err)
        } else {
            inv.CPUs = cpuDevices
        }
    }

    // --- 3. Get Memory (DIMMs) ---
    if dimmCollectionURI := systemData.Memory.ODataID; dimmCollectionURI != "" {
		cleanedURI := strings.TrimPrefix(dimmCollectionURI, "/redfish/v1")
        dimmDevices, err := getCollectionDevices(c, cleanedURI, "DIMM", systemURI, &RedfishMemory{})
        if err != nil {
            fmt.Printf("Warning: Failed to retrieve DIMM inventory from %s: %v\n", dimmCollectionURI, err)
        } else {
            inv.DIMMs = dimmDevices
        }
    }

	return inv, nil
}

// getCollectionDevices retrieves a collection, iterates over members, and maps them to DiscoveredDevice.
// componentTypeExample is an empty struct pointer (*RedfishProcessor, *RedfishMemory) used for typing.
func getCollectionDevices(c *RedfishClient, collectionURI, deviceType, parentURI string, componentTypeExample interface{}) ([]DiscoveredDevice, error) {
	var devices []DiscoveredDevice

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

		// Create a new instance of the correct component type for unmarshaling
        // We rely on the CommonRedfishProperties being the first embedded field
		component := reflect.New(reflect.TypeOf(componentTypeExample).Elem()).Interface()

		if err := json.Unmarshal(memberBody, &component); err != nil {
			fmt.Printf("Warning: Failed to unmarshal component %s: %v\n", member.ODataID, err)
			continue
		}
        
        // Use reflection to access the CommonRedfishProperties, which is the 0th field.
        // This is necessary because the component (Processor/Memory) struct is generic here.
		rfProps := reflect.ValueOf(component).Elem().Field(0).Interface().(CommonRedfishProperties)

		devices = append(devices, mapCommonProperties(rfProps, deviceType, member.ODataID, parentURI))
	}
	
	return devices, nil
}
