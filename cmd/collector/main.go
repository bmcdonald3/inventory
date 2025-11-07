package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/bmcdonald3/inventory-gather/pkg/inventory" 
)

var rootCmd = &cobra.Command{
	Use:   "redfish-inventory-collector",
	Short: "Gathers hardware inventory via Redfish and posts it to the OpenCHAMI API.",
	Run:   executeGatherAndPost,
}

var bmcIP string

func init() {
	// Define the --ip flag for the BMC IP
	rootCmd.Flags().StringVarP(&bmcIP, "ip", "i", "", "The IP address of the BMC to gather inventory from (required)")
	rootCmd.MarkFlagRequired("ip")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		// This will catch execution errors and print them clearly
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// executeGatherAndPost is the main function logic triggered by cobra.
func executeGatherAndPost(cmd *cobra.Command, args []string) {
	fmt.Printf("Starting inventory collection for BMC IP: %s\n", bmcIP)
	
	// Pass the IP captured by cobra to the core logic
	err := inventory.CollectAndPost(bmcIP) 
	if err != nil {
		// Log the error using cobra's framework
		fmt.Fprintf(os.Stderr, "Collection Failed: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("Inventory collection and posting completed successfully.")
}