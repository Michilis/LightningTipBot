package configuration

import (
	"log"
	"net/http"
	"os"
	"time"
)

// Config holds the application configuration
type Config struct {
	Cashu struct {
		ServiceURL string
	}
}

// Global configuration instance
var config = Config{}

// Get returns the global configuration instance
func Get() *Config {
	return &config
}

func init() {
	// Set default Cashu service URL
	config.Cashu.ServiceURL = "http://localhost:3333"
	
	// Override with environment variable if set
	if url := os.Getenv("CASHU_SERVICE_URL"); url != "" {
		config.Cashu.ServiceURL = url
	}

	// Check connection to Cashu service
	go func() {
		// Wait a bit for the service to start
		time.Sleep(2 * time.Second)
		
		// Try to connect to the health endpoint
		resp, err := http.Get(config.Cashu.ServiceURL + "/health")
		if err != nil {
			log.Printf("[Cashu] Warning: Could not connect to Cashu service at %s: %v", config.Cashu.ServiceURL, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			log.Printf("[Cashu] Successfully connected to Cashu service at %s", config.Cashu.ServiceURL)
		} else {
			log.Printf("[Cashu] Warning: Cashu service at %s returned status code %d", config.Cashu.ServiceURL, resp.StatusCode)
		}
	}()
} 