package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"syscall"
	"time"

	"golang.org/x/term"
)

const (
	serverAddr    = "https://example.com"                                       // domain that is checked for errors
	kvmPowerLong  = "https://pikvm.example.com/api/atx/click?button=power_long" // pikvm api endpoints for resetting the machine
	kvmPowerShort = "https://pikvm.example.com/api/atx/click?button=power"
)

type credentials struct {
	username string
	password string
}

func getCreds() (*credentials, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("[AUTH]: Username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("couldn't read username: %w", err)
	}
	username = username[:len(username)-1] // cut trailing newline

	fmt.Print("[AUTH]: Password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return nil, fmt.Errorf("couldn't read password: %w", err)
	}
	fmt.Println()

	return &credentials{
		username: username,
		password: string(passwordBytes),
	}, nil
}

func createHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
	}
}

func kvmPowerRequest(client *http.Client, creds *credentials, powerType string) error {
	req, err := http.NewRequest("POST", powerType, nil)
	if err != nil {
		return fmt.Errorf("couldn't create request: %w", err)
	}
	req.SetBasicAuth(creds.username, creds.password)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("couldn't send KVM request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

func resetServer(client *http.Client, creds *credentials) error {
	log.Println("[INFO] Resetting Server")
	// power off
	if err := kvmPowerRequest(client, creds, kvmPowerLong); err != nil {
		return fmt.Errorf("failed to power off machine: %w", err)
	}

	time.Sleep(5 * time.Second)
	// power back on
	if err := kvmPowerRequest(client, creds, kvmPowerShort); err != nil {
		return fmt.Errorf("failed to power on machine: %w", err)
	}

	log.Println("[INFO] Server Reset Successfully")
	return nil
}

func main() {
	creds, err := getCreds()
	if err != nil {
		log.Fatalf("[ERR] Error Getting Credentials: %v", err)
	}

	client := createHTTPClient()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	log.Println("[INFO] Starting Status Ping")
	for range ticker.C {
		req, err := http.NewRequest("GET", serverAddr, nil)
		if err != nil {
			log.Printf("[ERR] Failed to Create Ping Request: %v", err)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[ERR] Ping Request Failed: %v", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("[ERR] Server returns error: %d", resp.StatusCode)
			if err := resetServer(client, creds); err != nil {
				log.Printf("[ERR] Server Reset Error: %v", err)
			}
		}
		resp.Body.Close()
	}
}
