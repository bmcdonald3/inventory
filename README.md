# inventory-api

## Getting Started
This is an inventory API, generated with Fabrica, for OpenCHAMI based on the following data model:

### Core fields
* **id (UUID):** The permanent, unique identifier for the hardware.
* **deviceType (Enum):** The type of hardware (e.g., "Node", "GPU", "Rack").
* **manufacturer (String):** The manufacturer name.
* **partNumber (String):** The part number.
* **serialNumber (String):** The serial number.
* **parentID (UUID):** The parent device of this device. If null, this is a top-level device (i.e., a rack; dimms are children of nodes, etc.).
* **childrenDeviceIds (Array of UUIDs):** A read-only list of devices contained within this one. Calculated on-request, not stored (to avoid frequent updates).

### Arbitrary key-value store
* **properties (Map of strings to JSON values):** An arbitrary key-value map for storing additional, non-standard attributes.

<details><summary>Properties information</summary>

#### The `properties` Field for Custom Attributes
To resolve the open question regarding custom attributes, a `properties` field will be in the Device model. This field allows storing arbitrary key-value data that is not covered by the core model fields.

The `properties` field is a map where keys are strings and values can be any valid JSON type (string, number, boolean, null, array, or object). To ensure consistency and usability, the following constraints and guidelines apply.

#### Constraints on Keys
* all keys must be in **lowercase snake_case**.
* keys may only contain **lowercase alphanumeric characters** (a-z, 0-9), **underscores** (`_`), and **dots** (`.`).
* the dot character (`.`) is used exclusively as a **namespace separator** to group related attributes (e.g., `bios.release_date`).

#### Key Transformation Examples
| HPCM Key | OpenCHAMI Key |
| :--- | :--- |
| `biosBootMode` | `bios_boot_mode` |
| `operationalStatus` | `operational_status` |
| `rootFs` | `root_fs` |
| `CONSERVER_LOGGING` | `conserver_logging` |
| `dns_domain` | `dns_domain` |
| `Wake-up Type` | `wake_up_type` |
| `SKU Number` | `sku_number` |
| `bios.Release Date` | `bios.release_date` |

#### Other constraints
* all values stored in the `properties` field must be **valid JSON**
* whenever possible, use simple JSON types (String, Number, Boolean)
* only use JSON Objects or Arrays when data is inherently structured as a group or list

Example of a JSON Object value:
```json
"aliases": {
  "a2000": "ProLiant_XL225n_Gen10_Plus_n1",
  "brazos": "brazos1",
  "product": "XL225n_Gen10_Plus_n1"
}
```
Example of a JSON array value:
```json
"protocol": ["Hpe", "NO_DCMI", "NO_DCMI_NM", "None", "ipmi", "redfish"]
```
</details>

### Metadata
* **apiVersion (String):** The API group version (e.g., "inventory/v1").
* **kind (String):** The resource type (e.g., "Device").
* **schemaVersion (String):** The version of this resource's schema.
* **createdAt (Timestamp):** Timestamp of when the device was created.
* **updatedAt (Timestamp):** Timestamp of the last update.
* **deletedAt (Timestamp):** Timestamp for soft deletes.

---

## Usage

### Running the API Server
```bash
# Install dependencies
go mod tidy

# Run the server
go run ./cmd/server/
```

### Running the Redfish Collector
This repository includes a command-line tool, located at `cmd/collector/main.go`, to discover live hardware from a BMC via Redfish and populate the API. It uses the project's generated Go client SDK.

**Note:** The collector currently uses hardcoded credentials in `pkg/collector/collector.go` (`DefaultUsername` and `DefaultPassword`). These must be updated to match your target BMC.

**Command:**
```bash
# Run the collector, pointing it at a target BMC
go run ./cmd/collector/main.go --ip <BMC_IP_ADDRESS>
```

### Populating with Test Data (Alternative)
A shell script is available to populate the API with sample mock data.
```bash
# Run script with sample data to populate DB
./test-data/populate_node.sh
```

---

## Collector Verification and Results
The `redfish-inventory-collector` tool was successfully run against a live BMC (`172.24.0.2`), demonstrating a complete, end-to-end workflow from discovery to API submission.

### Execution Log (Verified Success)
The following output confirms that the refactored collector successfully connected, discovered 7 devices, and posted them hierarchically using the Fabrica-generated SDK.

```bash
# Command run from the inventory-api project root
$ go run ./cmd/collector/main.go --ip 172.24.0.2
```

```
Starting inventory collection for BMC IP: 172.24.0.2
Starting Redfish discovery...
Redfish Discovery Complete: Found 7 total devices.
-> Creating resource envelope for (Parent) Node-QSBP82909274...
-> Updating status for Node-QSBP82909274 (UID: dev-eda88642)...
-> Successfully posted parent device dev-eda88642
-> Creating resource envelope for (Child) CPU--Systems-QSBP82909274-Processors-CPU1...
-> Updating status for CPU--Systems-QSBP82909274-Processors-CPU1 (UID: dev-17ca1003)...
-> Successfully posted child device dev-17ca1003
-> Creating resource envelope for (Child) CPU--Systems-QSBP82909274-Processors-CPU2...
-> Updating status for CPU--Systems-QSBP82909274-Processors-CPU2 (UID: dev-1a19eb08)...
-> Successfully posted child device dev-1a19eb08
-> Creating resource envelope for (Child) DIMM-3128C51A...
-> Updating status for DIMM-3128C51A (UID: dev-8ce3321c)...
-> Successfully posted child device dev-8ce3321c
-> Creating resource envelope for (Child) DIMM-10CD71D4...
-> Updating status for DIMM-10CD71D4 (UID: dev-4f134c3c)...
-> Successfully posted child device dev-4f134c3c
-> Creating resource envelope for (Child) DIMM-3128C442...
-> Updating status for DIMM-3128C442 (UID: dev-7843e8af)...
-> Successfully posted child device dev-7843e8af
-> Creating resource envelope for (Child) DIMM-10CD71BE...
-> Updating status for DIMM-10CD71BE (UID: dev-a49acce5)...
-> Successfully posted child device dev-a49acce5
Inventory collection and posting completed successfully.
```

### API Data Structure Example
This section illustrates the complete JSON structure of the Node device (`Node-QSBP82909274`) posted by this tool, as stored in the API.

```json
{
  "apiVersion": "v1",
  "kind": "Device",
  "schemaVersion": "v1",
  "metadata": {
    "name": "Node-QSBP82909274",
    "uid": "dev-eda88642",
    "createdAt": "2025-11-07T09:41:00Z",
    "updatedAt": "2025-11-07T09:41:00Z"
  },
  "spec": {},
  "status": {
    "deviceType": "Node",
    "manufacturer": "Intel Corporation",
    "partNumber": "102072300",
    "serialNumber": "QSBP82909274",
    "properties": {
      "redfish_uri": "\"/Systems/QSBP82909274\""
    }
  }
}
```

### Data Posted to API
The successful posting included the following devices, demonstrating correct Redfish data extraction and UUID resolution for the `parentID` field:

| Device Type | API UID (Example) | Core Data | ParentID (Resolved UUID) |
| :--- | :--- | :--- | :--- |
| **Node** | `dev-eda88642` | Manufacturer: Intel Corporation, Serial: QSBP82909274 | (Empty, Top-Level) |
| **CPU** | `dev-17ca1003` | Manufacturer: Intel(R) Corporation, PartNumber: Intel Xeon processor | `dev-eda88642` |
| **DIMM** | `dev-8ce3321c` | Manufacturer: Hynix, Serial: 3128C51A | `dev-eda88642` |
| **DIMM** | `dev-4f134c3c` | Manufacturer: Hynix, Serial: 10CD71D4 | `dev-eda88642` |
