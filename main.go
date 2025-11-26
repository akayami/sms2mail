package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type GlobalConfig struct {
	ServerPort string `toml:"server_port"`
}

type ProfileConfig struct {
	EmailFrom    string `toml:"email_from"`
	EmailTo      string `toml:"email_to"`
	MsmtpProfile string `toml:"msmtp_profile"` // Optional: msmtp account name, defaults to "default"
}

var globalConfig GlobalConfig
var configDir string

func loadGlobalConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return toml.Unmarshal(data, &globalConfig)
}

func loadProfileConfig(profileName string) (*ProfileConfig, error) {
	// Construct path to profile config
	// Expecting sms2mail.d directory in the same location as the main config or current directory
	profilePath := filepath.Join(configDir, "sms2mail.d", profileName+".toml")

	// If configDir is empty (e.g. loaded from ./config.toml implicitly), try ./sms2mail.d
	if configDir == "" {
		profilePath = filepath.Join("sms2mail.d", profileName+".toml")
	}

	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile config '%s': %w", profilePath, err)
	}

	var profile ProfileConfig
	if err := toml.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse profile config '%s': %w", profilePath, err)
	}
	return &profile, nil
}

const configTemplate = `# Server settings
server_port = ":8080"
`
const profileTemplate = `# Email Configuration for profile
# Ensure this 'From' address is allowed by your msmtp configuration (provider)
email_from = "sms-notifier@yourserver.com"
email_to = "youremail@example.com"

# Optional: Specify which msmtp account/profile to use
# If not specified, defaults to "default"
# msmtp_profile = "default"
`

func main() {
	// Check for CLI arguments
	var explicitConfigPath string
	if len(os.Args) > 1 {
		if os.Args[1] == "config" {
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
		} else if os.Args[1] == "profileConfig" {
			if len(os.Args) > 2 {
				// Write to file
				err := os.WriteFile(os.Args[2], []byte(profileTemplate), 0644)
				if err != nil {
					log.Fatalf("Error writing profile config file: %v", err)
				}
				fmt.Printf("Profile configuration template written to %s\n", os.Args[2])
			} else {
				// Print to stdout
				fmt.Print(profileTemplate)
			}
			return
		}
		// Assume argument is a config file path
		explicitConfigPath = os.Args[1]
	}

	// Load configuration
	loadedPath, err := findAndLoadGlobalConfig(explicitConfigPath)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Store the directory of the loaded config to find profiles later
	configDir = filepath.Dir(loadedPath)
	if loadedPath == "config.toml" {
		configDir = "."
	}

	fmt.Printf("Loaded global configuration from %s: %+v\n", loadedPath, globalConfig)

	// Check if msmtp is installed and in the system PATH
	path, err := exec.LookPath("msmtp")
	if err != nil {
		log.Fatal("Error: 'msmtp' command not found in PATH. Please install it or check your environment.")
	}
	fmt.Printf("Found msmtp at: %s\n", path)

	// Handle /sms/ for profiles
	http.HandleFunc("/sms/", handleSMS)

	fmt.Printf("Service listening on %s...\n", globalConfig.ServerPort)
	if err := http.ListenAndServe(globalConfig.ServerPort, nil); err != nil {
		log.Fatal(err)
	}
}

func findAndLoadGlobalConfig(explicitPath string) (string, error) {
	// 1. Explicit path from CLI
	if explicitPath != "" {
		fmt.Printf("Loading config from explicit path: %s\n", explicitPath)
		return explicitPath, loadGlobalConfig(explicitPath)
	}

	// 2. /etc/sms2mail.toml
	if _, err := os.Stat("/etc/sms2mail.toml"); err == nil {
		fmt.Println("Found config at /etc/sms2mail.toml")
		return "/etc/sms2mail.toml", loadGlobalConfig("/etc/sms2mail.toml")
	}

	// 3. User Config Directory
	userConfigDir, err := os.UserConfigDir()
	if err == nil {
		userConfigPath := filepath.Join(userConfigDir, "sms2mail.toml")
		if _, err := os.Stat(userConfigPath); err == nil {
			fmt.Printf("Found config at %s\n", userConfigPath)
			return userConfigPath, loadGlobalConfig(userConfigPath)
		}
	}

	// 4. Current Directory
	if _, err := os.Stat("config.toml"); err == nil {
		fmt.Println("Found config at ./config.toml")
		return "config.toml", loadGlobalConfig("config.toml")
	}

	return "", fmt.Errorf("no configuration file found in /etc/sms2mail.toml, user config dir, or ./config.toml")
}

func handleSMS(w http.ResponseWriter, r *http.Request) {
	// Extract profile from URL path: /sms/<profile>
	path := r.URL.Path
	if !strings.HasPrefix(path, "/sms/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	profileName := strings.TrimPrefix(path, "/sms/")
	if profileName == "" {
		http.Error(w, "Profile name required", http.StatusBadRequest)
		return
	}

	// Load profile config
	profileConfig, err := loadProfileConfig(profileName)
	if err != nil {
		log.Printf("Error loading profile '%s': %v", profileName, err)
		http.Error(w, fmt.Sprintf("Profile '%s' not found or invalid", profileName), http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Extract Twilio data
	smsFrom := r.FormValue("From")
	smsBody := r.FormValue("Body")

	log.Printf("Received SMS from %s for profile '%s'. Forwarding via msmtp...", smsFrom, profileName)

	// Send via msmtp using profile config
	err = sendViaMsmtp(smsFrom, smsBody, profileConfig)
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

func sendViaMsmtp(phoneFrom, messageBody string, config *ProfileConfig) error {
	// Determine which msmtp profile/account to use
	msmtpProfile := config.MsmtpProfile
	if msmtpProfile == "" {
		msmtpProfile = "default"
	}

	// We use "msmtp" with the "-a" flag to specify the account and "-t" to read headers from stdin.
	// -a specifies the msmtp account/profile to use
	// -t tells msmtp to read the "To", "From", and "Subject" from the text we pipe in.
	cmd := exec.Command("msmtp", "-a", msmtpProfile, "-t")

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
