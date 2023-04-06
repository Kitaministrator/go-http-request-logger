package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	// Set the listening port range
	var ports []string
	ports, err := setPortRange()
	if err != nil {
		log.Printf("failed to set listening port range: %v", err)
		return
	}

	// Start listening and recording on the ports
	err = startRecordIncomings(ports)
	if err != nil {
		log.Printf("failed to start listening on ports: %v", err)
		return
	}

	// Wait for all servers to exit
	select {}
}

func setPortRange() ([]string, error) {

	var config struct {
		PortStart int `json:"portStart"`
		PortEnd   int `json:"portEnd"`
	}

	// Open the config file
	configFile, err := os.Open("config.json")
	if err != nil {
		if os.IsNotExist(err) {
			// Create a new config file if it doesn't exist with default values 8000-8001
			config.PortStart = 8000
			config.PortEnd = 8001
			cfg := make(map[string]int)
			cfg["portStart"] = config.PortStart
			cfg["portEnd"] = config.PortEnd
			jsonData, err := json.MarshalIndent(config, "", "  ")
			if err != nil {
				panic(err)
			}
			err = os.WriteFile("config.json", jsonData, 0644)
			if err != nil {
				panic(err)
			}
			log.Printf("config file created with/run with default values: %v", cfg)
		} else {
			return nil, err
		}
	} else { // Decode the JSON data from the file
		err = json.NewDecoder(configFile).Decode(&config)
		if err != nil {
			return nil, err
		}
	}
	defer configFile.Close()

	// Define the ports to listen on
	var ports []string

	for i := config.PortStart; i <= config.PortEnd; i++ {
		portString := strconv.Itoa(i)
		if portString == "" {
			return nil, fmt.Errorf("convert failed when handling port: %d", i)
		}
		ports = append(ports, portString)
	}

	if len(ports) == 0 {
		return nil, fmt.Errorf("no valid ports found in range")
	}

	return ports, nil
}

func startRecordIncomings(ports []string) error {
	// Create the log directory and define the log file name
	logPath := "logs/"
	logDate := time.Now().Format("20060102")
	logPathName := logPath + "log-" + logDate + ".json"
	err := os.MkdirAll(logPath, os.ModePerm)
	if err != nil {
		return err
	}

	// Start a goroutine for each port
	for _, port := range ports {
		go func(p string) {
			// Create a new HTTP server for the current port
			srv := &http.Server{Addr: ":" + p}

			// Handle requests on the server
			srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				// Create a map to store the request data
				reqData := make(map[string]interface{})
				reqData["body"] = string(body)
				reqData["headers"] = r.Header
				reqData["method"] = r.Method
				reqData["port"] = p

				// Encode the request data to JSON
				jsonData, err := json.Marshal(reqData)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				// Append the JSON data to a file
				file, err := os.OpenFile(logPathName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				defer file.Close()
				file.Write(jsonData)
				file.Write([]byte("\n")) // add a newline character

				// Respond to the request
				fmt.Fprintf(w, "Received request on port %s", p)
			})

			// Start the HTTP server for the current port
			log.Printf("Listening on port %s\n", p)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Failed to start server: %v", err)
			}
		}(port)
	}

	// Add a newline character at the end of the file
	file, err := os.OpenFile(logPathName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	file.Write([]byte("\n"))

	return nil
}
