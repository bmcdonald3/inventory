# inventory-api



## Getting Started

1. Define your resources in pkg/resources/
2. Generate code: fabrica generate
3. Run the server: go run cmd/server/main.go

## Configuration

The server supports configuration via:
- Command line flags
- Environment variables (INVENTORY-API_*)
- Configuration file (~/.inventory-api.yaml)

## Features

- 💾 File-based storage

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
```
