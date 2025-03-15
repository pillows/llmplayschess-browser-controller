package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// URLRequest represents the JSON payload with the URL to open
type URLRequest struct {
	URL string `json:"url"`
}

// Response represents the API response
type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// updateFirefoxURL changes the URL of the current Firefox tab
func updateFirefoxURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		// For Linux, we can use the Firefox remote protocol
		// First check if Firefox is running
		checkCmd := exec.Command("pgrep", "firefox")
		if err := checkCmd.Run(); err != nil {
			// Firefox is not running, start it with the URL
			cmd = exec.Command("firefox", url)
		} else {
			// Firefox is running, use xdotool to focus Firefox and simulate keystrokes
			// This approach is more reliable than --remote for modern Firefox
			focusCmd := exec.Command("xdotool", "search", "--onlyvisible", "--class", "Firefox", "windowactivate")
			if err := focusCmd.Run(); err != nil {
				return fmt.Errorf("failed to focus Firefox window: %v", err)
			}
			
			// Open a new tab with Ctrl+L to focus address bar, then type URL and press Enter
			selectCmd := exec.Command("xdotool", "key", "ctrl+l")
			if err := selectCmd.Run(); err != nil {
				return fmt.Errorf("failed to select address bar: %v", err)
			}
			
			// Type the URL (cleaner to split into two commands)
			typeCmd := exec.Command("xdotool", "type", "--clearmodifiers", url)
			if err := typeCmd.Run(); err != nil {
				return fmt.Errorf("failed to type URL: %v", err)
			}
			
			// Press Enter to navigate
			enterCmd := exec.Command("xdotool", "key", "Return")
			return enterCmd.Run()
		}
		
	case "darwin":
		// For macOS, we'll use AppleScript which is more reliable
		scriptContent := fmt.Sprintf(`
		tell application "Firefox"
			activate
			tell application "System Events"
				tell process "Firefox"
					keystroke "l" using command down
					delay 0.1
					keystroke "a" using command down
					delay 0.1
					keystroke "%s"
					delay 0.1
					keystroke return
				end tell
			end tell
		end tell`, url)
		cmd = exec.Command("osascript", "-e", scriptContent)
		
	case "windows":
		// For Windows, we'll use a PowerShell script
		// Check if Firefox is running
		checkCmd := exec.Command("tasklist", "/FI", "IMAGENAME eq firefox.exe", "/NH")
		output, _ := checkCmd.Output()
		if !strings.Contains(string(output), "firefox.exe") {
			// Firefox is not running, start it with the URL
			cmd = exec.Command("cmd", "/C", "start", "firefox.exe", url)
		} else {
			// Firefox is running, use PowerShell to focus and change URL
			psScript := fmt.Sprintf(`
			Add-Type -AssemblyName System.Windows.Forms
			# Focus Firefox window
			$firefox = Get-Process firefox | Where-Object {$_.MainWindowHandle -ne 0} | Select-Object -First 1
			if ($firefox) {
				[void][System.Reflection.Assembly]::LoadWithPartialName('Microsoft.VisualBasic')
				$hwnd = $firefox.MainWindowHandle
				[Microsoft.VisualBasic.Interaction]::AppActivate($hwnd)
				Start-Sleep -Milliseconds 100
				# Select address bar and enter URL
				[System.Windows.Forms.SendKeys]::SendWait("^l")
				Start-Sleep -Milliseconds 100
				[System.Windows.Forms.SendKeys]::SendWait("^a")
				Start-Sleep -Milliseconds 100
				[System.Windows.Forms.SendKeys]::SendWait("%s")
				Start-Sleep -Milliseconds 100
				[System.Windows.Forms.SendKeys]::SendWait("{ENTER}")
			} else {
				Start-Process "firefox.exe" -ArgumentList "%s"
			}`, url, url)
			cmd = exec.Command("powershell", "-Command", psScript)
		}
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	return cmd.Run()
}

func handleOpenURL(w http.ResponseWriter, r *http.Request) {
	// Set content type
	w.Header().Set("Content-Type", "application/json")

	// Only allow POST requests
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Message: "Only POST method is allowed",
		})
		return
	}

	// Decode the request
	var req URLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Message: "Invalid JSON payload",
		})
		return
	}

	// Validate URL
	if req.URL == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Message: "URL cannot be empty",
		})
		return
	}

	// Update URL in Firefox
	if err := updateFirefoxURL(req.URL); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Message: fmt.Sprintf("Failed to change URL: %v", err),
		})
		return
	}

	// Success response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Message: fmt.Sprintf("Successfully changed Firefox tab to %s", req.URL),
	})
}

func main() {
	// Get port from environment variable or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "9001"
	}

	// Register handlers
	http.HandleFunc("/open", handleOpenURL)

	// Start server
	addr := fmt.Sprintf(":%s", port)
	fmt.Printf("Server running on http://localhost%s\n", addr)
	fmt.Println("Send a POST request to /open with JSON payload {\"url\": \"https://example.com\"}")
	log.Fatal(http.ListenAndServe(addr, nil))
}
