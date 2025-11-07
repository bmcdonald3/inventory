// pkg/client/device_status.go
package client

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    // Import your canonical DeviceStatus struct
    "github.com/user/inventory-api/pkg/resources/device" 
)

// UpdateDeviceStatus performs a PUT /devices/{uid}/status request.
// This method is manually added to the client SDK.
func (c *DeviceClient) UpdateDeviceStatus(ctx context.Context, uid string, status *device.DeviceStatus) (*Device, error) {

    // Wrap the status in the expected { "status": { ... } } payload
    payload := map[string]interface{}{
        "status": status,
    }

    jsonPayload, err := json.Marshal(payload)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal status payload: %w", err)
    }

    // Build the URL for the /status sub-resource
    urlPath := fmt.Sprintf("/devices/%s/status", uid)

    req, err := c.client.NewRequest(ctx, http.MethodPut, urlPath, bytes.NewBuffer(jsonPayload))
    if err != nil {
        return nil, fmt.Errorf("failed to create UpdateStatus request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")

    var deviceResponse Device // Assumes SDK has a 'Device' response struct
    if err := c.client.Do(req, &deviceResponse); err != nil {
        return nil, fmt.Errorf("failed to execute UpdateStatus request: %w", err)
    }

    return &deviceResponse, nil
}