#!/bin/bash
set -euo pipefail

# --- Configuration ---
API_URL="http://localhost:8080" # Default Fabrica root
DATA_FILE="node_data.json"

# --- Pre-flight Checks ---
if ! command -v jq &> /dev/null; then
    echo "Error: jq is not installed. Please install it to run this script."
    exit 1
fi
if [ ! -f "$DATA_FILE" ]; then
    echo "Error: Data file not found at '$DATA_FILE'"
    exit 1
fi

JQ_SNAKE_CASE_FUNC='
# Converts "camelCase" to "snake_case"
def to_snake: gsub("(?<=[a-z0-9])(?=[A-Z])"; "_") | ascii_downcase;

# Recursively applies to_snake to all keys in an object or array
def to_snake_keys:
  if type == "object" then
    with_entries(.key |= to_snake | .value |= to_snake_keys)
  elif type == "array" then
    map(. | to_snake_keys)
  else
    .
  end
;
'

echo "--- Populating Inventory from $DATA_FILE ---"

# --- 1. Create Parent Node (Placeholder) ---
NODE_NAME=$(jq -r '.name' "$DATA_FILE")
echo "[1/4] Creating parent node placeholder: '$NODE_NAME'"

NODE_PAYLOAD=$(jq -n --arg name "$NODE_NAME" '{name: $name}')
NODE_RESPONSE=$(curl -s -X POST "${API_URL}/devices" -H "Content-Type: application/json" -d "$NODE_PAYLOAD")
NODE_DEV_UID=$(echo "$NODE_RESPONSE" | jq -r '.metadata.uid')

if [ -z "$NODE_DEV_UID" ] || [ "$NODE_DEV_UID" == "null" ]; then
    echo "Error: Failed to create parent node. Response:"
    echo "$NODE_RESPONSE"
    exit 1
fi
echo "    -> Parent Node created with UID: $NODE_DEV_UID"

# This array will store the UIDs of all created children
CHILD_UIDS=()

# --- 2. Create Child Components ---
echo "[2/4] Creating child components..."

# Create CPUs
echo "    -> Processing CPUs..."
for i in 1 2; do
    CPU_SERIAL=$(jq -r --arg key "cpu.Proc ${i}.Serial Number" '.inventory[$key] // empty' "$DATA_FILE")
    if [ -n "$CPU_SERIAL" ]; then
        CPU_NAME="${NODE_NAME}-cpu-proc${i}"
        CPU_MFG=$(jq -r --arg key "cpu.Proc ${i}.Manufacturer" '.inventory[$key]' "$DATA_FILE")
        CPU_VER=$(jq -r --arg key "cpu.Proc ${i}.Version" '.inventory[$key]' "$DATA_FILE")

        # 1. Create placeholder
        CHILD_PAYLOAD=$(jq -n --arg name "$CPU_NAME" '{name: $name}')
        CHILD_RESPONSE=$(curl -s -X POST "${API_URL}/devices" -H "Content-Type: application/json" -d "$CHILD_PAYLOAD")
        CHILD_UID=$(echo "$CHILD_RESPONSE" | jq -r '.metadata.uid')
        CHILD_UIDS+=("$CHILD_UID")

        # 2. Populate status
        STATUS_PAYLOAD=$(jq -n \
            --arg dt "CPU" \
            --arg mfg "$CPU_MFG" \
            --arg sn "$CPU_SERIAL" \
            --arg parent "$NODE_DEV_UID" \
            --arg ver "$CPU_VER" \
            '{
                "deviceType": $dt,
                "manufacturer": $mfg,
                "serialNumber": $sn,
                "partNumber": null,
                "parentID": $parent,
                "properties": {
                    "processor_id": "Proc '"$i"'",
                    "version": $ver
                }
            }')
        curl -s -X PUT "${API_URL}/devices/${CHILD_UID}/status" -H "Content-Type: application/json" -d "$STATUS_PAYLOAD" > /dev/null
        echo "        - Created CPU (UID: $CHILD_UID)"
    fi
done

# Create DIMMs
echo "    -> Processing DIMMs..."
DIMM_IDS=$(jq -r '.inventory | keys[] | select(startswith("dimm.")) | split(".")[1]' "$DATA_FILE" | sort -u)
for id in $DIMM_IDS; do
    DIMM_SERIAL=$(jq -r --arg key "dimm.${id}.Serial Number" '.inventory[$key]' "$DATA_FILE")
    DIMM_MFG=$(jq -r --arg key "dimm.${id}.Manufacturer" '.inventory[$key]' "$DATA_FILE")
    DIMM_NAME="${NODE_NAME}-dimm-${id}"

    # 1. Create placeholder
    CHILD_PAYLOAD=$(jq -n --arg name "$DIMM_NAME" '{name: $name}')
    CHILD_RESPONSE=$(curl -s -X POST "${API_URL}/devices" -H "Content-Type: application/json" -d "$CHILD_PAYLOAD")
    CHILD_UID=$(echo "$CHILD_RESPONSE" | jq -r '.metadata.uid')
    CHILD_UIDS+=("$CHILD_UID")

    # 2. Populate status
    STATUS_PAYLOAD=$(jq -n \
        --arg dt "DIMM" \
        --arg mfg "$DIMM_MFG" \
        --arg sn "$DIMM_SERIAL" \
        --arg parent "$NODE_DEV_UID" \
        '{
            "deviceType": $dt,
            "manufacturer": $mfg,
            "serialNumber": $sn,
            "partNumber": null,
            "parentID": $parent,
            "properties": {
                "dimm_id": "'"$id"'"
            }
        }')
    curl -s -X PUT "${API_URL}/devices/${CHILD_UID}/status" -H "Content-Type: application/json" -d "$STATUS_PAYLOAD" > /dev/null
    echo "        - Created DIMM (UID: $CHILD_UID)"
done

# Create NICs (from network.nics array)
echo "    -> Processing NICs..."
jq -c '.network.nics[]' "$DATA_FILE" | while read -r nic_json; do
    NIC_NAME=$(echo "$nic_json" | jq -r '.name')
    NIC_MAC=$(echo "$nic_json" | jq -r '.macAddress')
    NIC_MFG=$(jq -r --arg key "nic.${NIC_NAME}.manufacturer" '.inventory[$key] // "Unknown"' "$DATA_FILE")

    # 1. Create placeholder
    CHILD_PAYLOAD=$(jq -n --arg name "${NODE_NAME}-${NIC_NAME}" '{name: $name}')
    CHILD_RESPONSE=$(curl -s -X POST "${API_URL}/devices" -H "Content-Type: application/json" -d "$CHILD_PAYLOAD")
    CHILD_UID=$(echo "$CHILD_RESPONSE" | jq -r '.metadata.uid')
    CHILD_UIDS+=("$CHILD_UID")

    # 2. Populate status
    STATUS_PAYLOAD=$(jq -n \
        --arg dt "NIC" \
        --arg mfg "$NIC_MFG" \
        --arg sn "$NIC_MAC" \
        --arg parent "$NODE_DEV_UID" \
        --argjson props_in "$nic_json" \
        "
        $JQ_SNAKE_CASE_FUNC
        
        {
            \"deviceType\": \$dt,
            \"manufacturer\": \$mfg,
            \"serialNumber\": \$sn,
            \"partNumber\": null,
            \"parentID\": \$parent,
            \"properties\": (\$props_in | to_snake_keys)
        }
        ")
    curl -s -X PUT "${API_URL}/devices/${CHILD_UID}/status" -H "Content-Type: application/json" -d "$STATUS_PAYLOAD" > /dev/null
    echo "        - Created NIC (UID: $CHILD_UID)"
done

# Create Disks
echo "    -> Processing Disks..."
DISK_IDS=$(jq -r '.inventory | keys[] | select(startswith("disk.")) | split(".")[1]' "$DATA_FILE" | sort -u)
for id in $DISK_IDS; do
    DISK_SERIAL=$(jq -r --arg key "disk.${id}.serial_number" '.inventory[$key]' "$DATA_FILE")
    DISK_NAME="${NODE_NAME}-disk-${id}"

    # 1. Create placeholder
    CHILD_PAYLOAD=$(jq -n --arg name "$DISK_NAME" '{name: $name}')
    CHILD_RESPONSE=$(curl -s -X POST "${API_URL}/devices" -H "Content-Type: application/json" -d "$CHILD_PAYLOAD")
    CHILD_UID=$(echo "$CHILD_RESPONSE" | jq -r '.metadata.uid')
    CHILD_UIDS+=("$CHILD_UID")

    # 2. Populate status
    STATUS_PAYLOAD=$(jq -n \
        --arg dt "Disk" \
        --arg sn "$DISK_SERIAL" \
        --arg parent "$NODE_DEV_UID" \
        '{
            "deviceType": $dt,
            "manufacturer": null,
            "serialNumber": $sn,
            "partNumber": null,
            "parentID": $parent,
            "properties": {
                "disk_id": "'"$id"'"
            }
        }')
    curl -s -X PUT "${API_URL}/devices/${CHILD_UID}/status" -H "Content-Type: application/json" -d "$STATUS_PAYLOAD" > /dev/null
    echo "        - Created Disk (UID: $CHILD_UID)"
done


# --- 3. Populate Parent Node Status ---
echo "[3/4] Populating parent node's status..."

CHILD_UIDS_JSON=$(printf '%s\n' "${CHILD_UIDS[@]}" | jq -R . | jq -s .)

NODE_STATUS_PAYLOAD=$(jq -n \
    --arg dt "Node" \
    --argjson c_uids "$CHILD_UIDS_JSON" \
    --argjson file_content "$(cat $DATA_FILE)" \
"
# --- Define Key Transformation Functions ---
$JQ_SNAKE_CASE_FUNC

def to_inventory_key:
  gsub(\" \"; \"_\") | # \"Product Name\" -> \"Product_Name\"
  gsub(\"(?<=[a-z0-9])(?=[A-Z])\"; \"_\") | # \"FirmwareVersion\" -> \"Firmware_Version\"
  ascii_downcase
;

# --- Build the Payload ---
{
  \"deviceType\": \$dt,
  \"manufacturer\": \$file_content.inventory[\"sys.Manufacturer\"],
  \"serialNumber\": \$file_content.inventory[\"sys.Serial Number\"],
  \"partNumber\": \$file_content.inventory[\"fru.system.SKU\"],
  \"parentID\": null,
  \"childrenDeviceIds\": \$c_uids,
  \"properties\": {
    \"old_uuid\": \$file_content.uuid,
    \"aliases\": \$file_content.aliases | to_snake_keys,
    \"network\": \$file_content.network | to_snake_keys,
    \"image\": \$file_content.image | to_snake_keys,
    \"platform\": \$file_content.platform | to_snake_keys,
    \"management\": \$file_content.management | to_snake_keys,
    \"attributes\": \$file_content.attributes | to_snake_keys,
    \"inventory\": \$file_content.inventory
                  | del(.[keys[] | select(
                        startswith(\"cpu.\") or
                        startswith(\"dimm.\") or
                        startswith(\"nic.\") or
                        startswith(\"disk.\")
                    )])
                  | with_entries(.key |= to_inventory_key)
  }
}
")

curl -s -X PUT "${API_URL}/devices/${NODE_DEV_UID}/status" \
    -H "Content-Type: application/json" \
    -d "$NODE_STATUS_PAYLOAD" > /dev/null

echo "[4/4] Parent node status updated with children and properties."
echo "--- Population complete! ---"
echo "Test by running: curl -s ${API_URL}/devices/${NODE_DEV_UID} | jq"