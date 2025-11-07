package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/openchami/fabrica/pkg/events"
	"github.com/openchami/fabrica/pkg/reconcile"
	// Import the base storage interface
	fabrica_storage "github.com/openchami/fabrica/pkg/storage"
	"github.com/user/inventory-api/pkg/reconcilers"

	// Import the GENERATED storage implementation
	internal_storage "github.com/user/inventory-api/internal/storage"

	// <<< FIX: Import the GENERATED events package
	internal_events "github.com/user/inventory-api/internal/middleware"
)

// --- Global variables for handlers ---
var (
	// Use the aliased interface type
	globalStorage fabrica_storage.StorageBackend
	globalEventBus events.EventBus
)

// SetStorageBackend sets the global storage backend
func SetStorageBackend(s fabrica_storage.StorageBackend) {
	globalStorage = s
}

// SetEventBus sets the global event bus
func SetEventBus(eb events.EventBus) {
	globalEventBus = eb
}
// --- End global variables ---


// Config holds all configuration for the service
type Config struct {
	// Server Configuration
	Port         int    `mapstructure:"port"`
	Host         string `mapstructure:"host"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
	IdleTimeout  int    `mapstructure:"idle_timeout"`

	// Storage Configuration
	DataDir string `mapstructure:"data_dir"`

	// Feature Flags
	Debug bool `mapstructure:"debug"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Port:         8080,
		Host:         "0.0.0.0",
		ReadTimeout:  15,
		WriteTimeout: 15,
		IdleTimeout:  60,
		DataDir:      "./data",
		Debug:        false,
	}
}

var (
	cfgFile string
	config  *Config
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:   "inventory-api",
	Short: "",
	Long:  `inventory-api - A Fabrica-generated OpenCHAMI service`,
	RunE:  runServer,
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the inventory-api server",
	Long:  `Start the inventory-api HTTP server with the configured options`,
	RunE:  runServer,
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.inventory-api.yaml)")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")

	// Server flags
	serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	serveCmd.Flags().String("host", "0.0.0.0", "Host to bind to")
	serveCmd.Flags().Int("read-timeout", 15, "Read timeout in seconds")
	serveCmd.Flags().Int("write-timeout", 15, "Write timeout in seconds")
	serveCmd.Flags().Int("idle-timeout", 60, "Idle timeout in seconds")

	serveCmd.Flags().String("data-dir", "./data", "Directory for file storage")

	// Bind flags to viper
	viper.BindPFlags(serveCmd.Flags())
	viper.BindPFlags(rootCmd.PersistentFlags())

	// Add subcommands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)
}

func initConfig() {
	config = DefaultConfig()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in home directory
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".inventory-api")
	}

	// Environment variables
	viper.SetEnvPrefix("INVENTORY-API")
	viper.AutomaticEnv()

	// Read config file if it exists
	if err := viper.ReadInConfig(); err == nil {
		log.Printf("Using config file: %s", viper.ConfigFileUsed())
	}

	// Unmarshal config
	if err := viper.Unmarshal(config); err != nil {
		log.Fatalf("Unable to decode into config struct: %v", err)
	}

	// Set debug logging
	if config.Debug {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("Debug logging enabled")
	}
}

func runServer(cmd *cobra.Command, args []string) error {
	log.Printf("Starting inventory-api server...")

	// --- 1. Initialize Storage Backend ---
	if err := internal_storage.InitFileBackend(config.DataDir); err != nil {
		return fmt.Errorf("failed to initialize file storage: %w", err)
	}
	
	storageBackend := internal_storage.Backend 
	if storageBackend == nil {
		 return fmt.Errorf("storage backend is nil after initialization")
	}

	SetStorageBackend(storageBackend) // Set for global access
	log.Printf("File storage initialized in %s", config.DataDir)


	// --- 2. Initialize Event Bus ---
	// <<< FIX: Use the GENERATED event bus initializer and instance
	log.Println("Initializing generated in-memory event bus...")
	if err := internal_events.InitializeEventBus(); err != nil {
		return fmt.Errorf("failed to initialize event bus: %w", err)
	}
	eventBus := internal_events.GlobalEventBus
	if eventBus == nil {
		return fmt.Errorf("event bus is nil after initialization")
	}
	// The generated bus is started by its initializer
	defer internal_events.CloseEventBus() // Use the generated closer
	SetEventBus(eventBus) // Set global variable (may be redundant, but safe)
	// <<< END FIX


	// --- 3. Initialize Reconciliation Controller ---
	log.Println("Initializing reconciliation controller...")
	// Pass the *same* eventBus to the controller
	controller := reconcile.NewController(eventBus, storageBackend)

	
	// --- 4. Register Reconcilers ---
	log.Println("Registering reconcilers...")
	snapshotReconciler := &reconcilers.DiscoverySnapshotReconciler{
		BaseReconciler: reconcile.BaseReconciler{
			EventBus: eventBus,
			Logger:   reconcile.NewDefaultLogger(), // Simple logger
		},
		Storage: storageBackend, // Give it access to the *same* storage
	}
	controller.RegisterReconciler(snapshotReconciler)
	log.Printf("Registered reconciler for %s", snapshotReconciler.GetResourceKind())


	// --- 5. Start Controller ---
	controllerCtx, controllerCancel := context.WithCancel(context.Background())
	
	go func() {
		log.Println("Reconciliation controller starting.")
		if err := controller.Start(controllerCtx); err != nil {
			log.Printf("Reconciliation controller error: %v", err)
		}
	}()


	// --- 6. Setup Router ---
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	if config.Debug {
		r.Mount("/debug", middleware.Profiler())
	}
	// This function registers the handlers that use the generated storage
	RegisterGeneratedRoutes(r) 
	r.Get("/health", healthHandler)

	
	// --- 7. Create and Start HTTP Server ---
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  time.Duration(config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(config.IdleTimeout) * time.Second,
	}

	go func() {
		log.Printf("Server starting on %s", addr)
		log.Printf("Storage: file backend in %s", config.DataDir)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	
	// --- 8. Wait for Interrupt (Graceful Shutdown) ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Server shutting down...")

	// --- 9. Shut Down Controller ---
	log.Println("Signaling reconciliation controller to stop...")
	controllerCancel() // Signal context to be done
	controller.Stop()    // Wait for work queue to empty
	log.Println("Reconciliation controller stopped.")


	// --- 10. Shut Down HTTP Server ---
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	log.Println("Server exited")
	return nil
}

// Health check handler
func healthHandler(w http.Writerr, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","service":"inventory-api"}`))
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Long:  `Print the version number of inventory-api`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("inventory-api v1.0.0")
	},
}