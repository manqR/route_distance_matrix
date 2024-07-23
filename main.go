package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

// DistanceMatrixResponse represents the response from the Google Distance Matrix API
type DistanceMatrixResponse struct {
	Rows []struct {
		Elements []struct {
			Distance struct {
				Text  string `json:"text"`
				Value int    `json:"value"`
			} `json:"distance"`
			Duration struct {
				Text  string `json:"text"`
				Value int    `json:"value"`
			} `json:"duration"`
			Status string `json:"status"`
		} `json:"elements"`
	} `json:"rows"`
	Status string `json:"status"`
}

func getDistanceMatrix(apiKey, origin, destination string) (*DistanceMatrixResponse, error) {
	mode := "driving"
	baseURL := "https://maps.googleapis.com/maps/api/distancematrix/json"
	params := url.Values{}
	params.Add("origins", origin)
	params.Add("destinations", destination)
	params.Add("mode", mode) 
	params.Add("key", apiKey)

	resp, err := http.Get(fmt.Sprintf("%s?%s", baseURL, params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var distanceMatrix DistanceMatrixResponse
	err = json.Unmarshal(body, &distanceMatrix)
	if err != nil {
		return nil, err
	}

	if distanceMatrix.Status != "OK" {
		return nil, fmt.Errorf("API error: %s", distanceMatrix.Status)
	}

	return &distanceMatrix, nil
}

func readCoordinatesFromCSV(filename string) ([][2]string, []string, []string, []string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if len(records) < 2 {
		return nil, nil, nil, nil, fmt.Errorf("CSV file must contain at least two rows")
	}

	var coordinates [][2]string
	var siteCodes []string
	var siteNames []string
	var terminalCodes []string

	for i, record := range records[1:] {
		if len(record) < 7 {
			return nil, nil, nil, nil, fmt.Errorf("CSV row %d has insufficient columns", i+2)
		}
		origin := fmt.Sprintf("%s,%s", record[5], record[6])
		destination := fmt.Sprintf("%s,%s", record[2], record[3])
		coordinates = append(coordinates, [2]string{origin, destination})
		siteCodes = append(siteCodes, record[0])
		siteNames = append(siteNames, record[1])
		terminalCodes = append(terminalCodes, record[4])
	}

	return coordinates, siteCodes, siteNames, terminalCodes, nil
}

func writeResultsToCSV(filename string, siteCodes []string, siteNames []string, terminalCodes []string, distances []float64, durations []string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"SITE_CODE", "SITE_NAME", "TERMINAL_CODE", "DISTANCE_KM", "DURATION"}); err != nil {
		return err
	}

	// Write records
	for i, code := range siteCodes {
		record := []string{code, siteNames[i], terminalCodes[i], fmt.Sprintf("%.2f", distances[i]), durations[i]}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
		os.Exit(1)
	}

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		fmt.Println("Error: GOOGLE_API_KEY environment variable is not set.")
		os.Exit(1)
	}

	// Read coordinates from CSV file
	coordinates, siteCodes, siteNames, terminalCodes, err := readCoordinatesFromCSV("routes.csv")
	if err != nil {
		fmt.Printf("Error reading coordinates from CSV: %v\n", err)
		os.Exit(1)
	}

	var distances []float64
	var durations []string

	// Process each origin-destination pair
	for _, pair := range coordinates {
		origin := pair[0]
		destination := pair[1]

		// Fetch distance matrix
		distanceMatrix, err := getDistanceMatrix(apiKey, origin, destination)
		if err != nil {
			fmt.Printf("Error fetching distance matrix for origin %s and destination %s: %v\n", origin, destination, err)
			distances = append(distances, 0) // Append 0 for error cases
			durations = append(durations, "N/A")
			continue
		}

		// Extract and store distance and duration
		if len(distanceMatrix.Rows) > 0 && len(distanceMatrix.Rows[0].Elements) > 0 {
			distance := float64(distanceMatrix.Rows[0].Elements[0].Distance.Value) / 1000 // Convert meters to kilometers
			duration := distanceMatrix.Rows[0].Elements[0].Duration.Text
			distances = append(distances, distance)
			durations = append(durations, duration)
		} else {
			distances = append(distances, 0) // Append 0 if no distance information is available
			durations = append(durations, "N/A")
		}
	}

	// Write results to CSV file
	if err := writeResultsToCSV("output.csv", siteCodes, siteNames, terminalCodes, distances, durations); err != nil {
		fmt.Printf("Error writing results to CSV: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Results have been written to output.csv")
}
