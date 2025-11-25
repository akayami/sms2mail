package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	ServerPort string `toml:"server_port"`
	EmailFrom  string `toml:"email_from"`
	EmailTo    string `toml:"email_to"`
}

var config Config

func loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return toml.Unmarshal(data, &config)
}

const configTemplate = `# Server settings
server_port = ":8080"

# Email Configuration
# Ensure this 'From' address is allowed by your msmtp configuration (provider)
email_from = "sms-notifier@yourserver.com"
email_to = "youremail@example.com"
`

func main() {
	// Check for CLI arguments
	if len(os.Args) > 1 && os.Args[1] == "config" {
		if len(os.Args) > 2 {
			// Write to file
			err := os.WriteFile(os.Args[2], []byte(configTemplate), 0644)
			if err != nil {
				log.Fatalf("Error writing config file: %v", err)
			}
			fmt.Printf("Configuration template written to %s\n", os.Args[2])
		} else {
			// Print to stdout
			fmt.Print(configTemplate)
		}
		return
	}

	// Load configuration
	if err := loadConfig("config.toml"); err != nil {
		log.Fatalf("Error loading config.toml: %v", err)
	}
	fmt.Printf("Loaded configuration: %+v\n", config)

	// Check if msmtp is installed and in the system PATH
	path, err := exec.LookPath("msmtp")
	if err != nil {
		log.Fatal("Error: 'msmtp' command not found in PATH. Please install it or check your environment.")
	}
	fmt.Printf("Found msmtp at: %s\n", path)

	http.HandleFunc("/sms", handleSMS)

	fmt.Printf("Service listening on %s...\n", config.ServerPort)
	if err := http.ListenAndServe(config.ServerPort, nil); err != nil {
		log.Fatal(err)
	}
}

func handleSMS(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Extract Twilio data
	smsFrom := r.FormValue("From")
	smsBody := r.FormValue("Body")

	log.Printf("Received SMS from %s. Forwarding via msmtp...", smsFrom)

	// Send via msmtp
	err := sendViaMsmtp(smsFrom, smsBody)
	if err != nil {
		log.Printf("Error sending email: %v", err)
		// Return 200 to Twilio so they don't retry, but log the error locally
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Println("Email handed off to msmtp successfully.")

	// Return empty TwiML to Twilio
	w.Header().Set("Content-Type", "text/xml")
	w.Write([]byte(`<Response></Response>`))
}

func sendViaMsmtp(phoneFrom, messageBody string) error {
	// We use "msmtp" with the "-t" flag.
	// -t tells msmtp to read the "To", "From", and "Subject" from the text we pipe in.
	cmd := exec.Command("msmtp", "-t")

	// Create the email content
	var emailContent bytes.Buffer
	fmt.Fprintf(&emailContent, "To: %s\n", config.EmailTo)
	fmt.Fprintf(&emailContent, "From: %s\n", config.EmailFrom)
	fmt.Fprintf(&emailContent, "Subject: SMS from %s\n", phoneFrom)
	fmt.Fprintf(&emailContent, "MIME-Version: 1.0\n")
	fmt.Fprintf(&emailContent, "Content-Type: text/plain; charset=\"UTF-8\"\n")
	fmt.Fprintf(&emailContent, "\n") // Blank line separates headers from body
	fmt.Fprintf(&emailContent, "You received a new SMS from: %s\n\n", phoneFrom)
	fmt.Fprintf(&emailContent, "---\n%s\n---", messageBody)

	// Pipe the content to msmtp's Standard Input
	cmd.Stdin = &emailContent

	// Execute
	// Capture output in case msmtp fails (useful for debugging config errors)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("msmtp failed: %v, output: %s", err, output)
	}

	return nil
}
