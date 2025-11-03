# inventory-api

## Getting Started

This is an inventory API, generated with Fabrica, for OpenCHAMI based on the following data model:

**Core fields**
* `id` (UUID): The permanent, unique identifier for the hardware.
* `deviceType` (Enum): The type of hardware (e.g., "Node", "GPU", "Rack").
* `manufacturer` (String): The manufacturer name.
* `partNumber` (String): The part number.
* `serialNumber` (String): The serial number.
* `parentID` (UUID): The parent device of this device. If null, this is a top-level device (i.e., a `rack`; dimms are children of nodes, etc.).
* `childrenDeviceIds` (Array of UUIDs): A read-only list of devices contained within this one. Calculated on-request, not stored (to avoid frequent updates).

**Arbitrary key-value store**
* `properties` (Map of strings to JSON values): An arbitrary key-value map for storing additional, non-standard attributes.

<details><summary>Properties information</summary>

### The `properties` Field for Custom Attributes

To resolve the open question regarding custom attributes, a `properties` field will be in the `Device` model. This field allows storing arbitrary key-value data that is not covered by the core model fields.

The `properties` field is a map where keys are strings and values can be any valid JSON type (string, number, boolean, null, array, or object). To ensure consistency and usability, the following constraints and guidelines apply.

#### Constraints on Keys

* all keys must be in lowercase `snake_case`.
* keys may only contain lowercase alphanumeric characters (`a-z`, `0-9`), underscores (`_`), and dots (`.`).
* the dot character (`.`) is used exclusively as a namespace separator to group related attributes (e.g., `bios.release_date`).

#### Key Transformation Examples

| HPCM Key            | OpenCHAMI Key       |
| ------------------- | ------------------- |
| `biosBootMode`      | `bios_boot_mode`    |
| `operationalStatus` | `operational_status`|
| `rootFs`            | `root_fs`           |
| `CONSERVER_LOGGING` | `conserver_logging` |
| `dns_domain`        | `dns_domain`        |
| `Wake-up Type`      | `wake_up_type`      |
| `SKU Number`        | `sku_number`        |
| `bios.Release Date` | `bios.release_date` |

#### Other constraints
* all values stored in the `properties` field must be valid JSON
* whenever possible, use simple JSON types (String, Number, Boolean)
* only use JSON Objects or Arrays when data is inherently structured as a group or list

**Example of a JSON Object value:**
```json
"aliases": {
  "a2000": "ProLiant_XL225n_Gen10_Plus_n1",
  "brazos": "brazos1",
  "product": "XL225n_Gen10_Plus_n1"
}
```

**Example of a JSON array value:**
```json
"protocol": ["Hpe", "NO_DCMI", "NO_DCMI_NM", "None", "ipmi", "redfish"]
```

</details>

**Metadata**
* `apiVersion` (String): The API group version (e.g., "inventory/v1").
* `kind` (String): The resource type (e.g., "Device").
* `schemaVersion` (String): The version of this resource's schema.
* `createdAt` (Timestamp): Timestamp of when the device was created.
* `updatedAt` (Timestamp): Timestamp of the last update.
* `deletedAt` (Timestamp): Timestamp for soft deletes.

## Testing

### Launch the server
```bash
# Install dependencies
go mod tidy

# Run the server
go run ./cmd/server/
```

### Populate with test data
```bash
# Run script with sample data to populate DB
./test-data/populate_node.sh

# Script will tell you what to run next...
# Explore the generated data by querying devices!
```
