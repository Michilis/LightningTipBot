package cashu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
)

// RedeemResponse represents the response from the Cashu redeem service
type RedeemResponse struct {
	Success     bool   `json:"success"`
	Amount      int64  `json:"amount"`
	Error       string `json:"error,omitempty"`
	MintURL     string `json:"mint_url"`
	Fee         int64  `json:"fee"`
	TotalAmount int64  `json:"total_amount"`
	NetAmount   int64  `json:"net_amount"`
}

// ContainsCashuToken checks if a message contains a Cashu token
func ContainsCashuToken(msg string) bool {
	// Match both v1 and v3 token formats
	matched, _ := regexp.MatchString(`(?i)cashu[abAB][a-zA-Z0-9-_]+`, msg)
	return matched
}

// ExtractCashuToken extracts the Cashu token from a message
func ExtractCashuToken(msg string) string {
	// Extract the full token including any potential whitespace
	re := regexp.MustCompile(`(?i)cashu[abAB][a-zA-Z0-9-_]+`)
	match := re.FindString(msg)
	return strings.TrimSpace(match)
}

// RedeemCashuToken sends a token to the Cashu service for redemption
func RedeemCashuToken(token string, serviceURL string) (int64, error) {
	log.Printf("[Cashu] Attempting to redeem token at %s", serviceURL)

	// Create request body
	body, err := json.Marshal(map[string]string{"token": token})
	if err != nil {
		log.Printf("[Cashu] Error marshaling request: %v", err)
		return 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request to Cashu service
	resp, err := http.Post(serviceURL+"/redeem", "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("[Cashu] Error sending request to service: %v", err)
		return 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var result RedeemResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[Cashu] Error decoding response: %v", err)
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for error in response
	if !result.Success {
		if result.Error != "" {
			log.Printf("[Cashu] Token redemption failed: %s", result.Error)
			return 0, fmt.Errorf("token redemption failed: %s", result.Error)
		}
		log.Printf("[Cashu] Token redemption failed without specific error")
		return 0, fmt.Errorf("token redemption failed")
	}

	log.Printf("[Cashu] Successfully redeemed token for %d sats", result.Amount)
	return result.Amount, nil
}

// PayInvoice sends an invoice to the Cashu service to be paid by the mint
func PayInvoice(paymentRequest string, serviceURL string) error {
	// Create request body
	requestBody := struct {
		PaymentRequest string `json:"payment_request"`
	}{
		PaymentRequest: paymentRequest,
	}

	// Marshal request body
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("error marshaling request: %v", err)
	}

	// Send request to Cashu service
	resp, err := http.Post(serviceURL+"/pay", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("error from Cashu service: %s", string(body))
	}

	return nil
}

// GetRedeemResponse gets the full redemption response from the Cashu service
func GetRedeemResponse(token string, serviceURL string) (*RedeemResponse, error) {
	log.Printf("[Cashu] Getting redemption response for token at %s", serviceURL)

	// Create request body
	body, err := json.Marshal(map[string]string{"token": token})
	if err != nil {
		log.Printf("[Cashu] Error marshaling request: %v", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request to Cashu service
	resp, err := http.Post(serviceURL+"/redeem", "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("[Cashu] Error sending request to service: %v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var result RedeemResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[Cashu] Error decoding response: %v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for error in response
	if !result.Success {
		if result.Error != "" {
			log.Printf("[Cashu] Token redemption failed: %s", result.Error)
			return nil, fmt.Errorf("token redemption failed: %s", result.Error)
		}
		log.Printf("[Cashu] Token redemption failed without specific error")
		return nil, fmt.Errorf("token redemption failed")
	}

	log.Printf("[Cashu] Successfully got redemption response for %d sats", result.Amount)
	return &result, nil
} 