package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/olekukonko/tablewriter"
)

// PackageInfo holds information about a package's size
type PackageInfo struct {
	Name    string
	Size    string
	RawSize int
}

// PackageMeta holds metadata about an npm package. It contains information such as the latest version and its corresponding distribution
// tags, as well as version-specific details including the size of the unpacked distribution.
type PackageMeta struct {
	DistTags struct {
		Latest string `json:"latest"`
	} `json:"dist-tags"`
	Versions map[string]struct {
		Dist struct {
			UnpackedSize int `json:"unpackedSize"`
		} `json:"dist"`
	} `json:"versions"`
}

// formatBytes converts bytes to a human-readable string
func formatBytes(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d Bytes", bytes)
	}
	div, exp := unit, 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// fetchPackageSize fetches the size of the latest version of a package
func fetchPackageSize(pkgName, token string, wg *sync.WaitGroup, results chan<- PackageInfo) {
	defer wg.Done()
	apiURL := fmt.Sprintf("https://registry.npmjs.org/%s", pkgName)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Printf("Failed to create request for %s: %v", pkgName, err)
		return
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch %s: %v", pkgName, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Package %s not found (status: %d)", pkgName, resp.StatusCode)
		return
	}

	var meta PackageMeta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		log.Printf("Failed to decode response for %s: %v", pkgName, err)
		return
	}

	latest := meta.DistTags.Latest
	version, exists := meta.Versions[latest]
	if !exists {
		log.Printf("Latest version not found for %s", pkgName)
		return
	}

	size := version.Dist.UnpackedSize
	results <- PackageInfo{
		Name:    pkgName,
		Size:    formatBytes(size),
		RawSize: size,
	}
}

// writeCSV writes package size information to a CSV file
func writeCSV(filePath string, data []PackageInfo) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{"Package Name", "Size (Bytes)", "Size (Pretty)"}); err != nil {
		return err
	}

	for _, pkg := range data {
		if err := writer.Write([]string{pkg.Name, fmt.Sprintf("%d", pkg.RawSize), pkg.Size}); err != nil {
			return err
		}
	}
	return nil
}

// printTable prints package size information as a table in the console
func printTable(data []PackageInfo) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Package Name", "Size (Bytes)", "Size (Pretty)"})
	for _, pkg := range data {
		table.Append([]string{pkg.Name, fmt.Sprintf("%d", pkg.RawSize), pkg.Size})
	}
	table.Render()
}

// getNpmToken retrieves the npm auth token from ~/.npmrc
// It reads the contents of the ~/.npmrc file and searches for a line that starts with
// "//registry.npmjs.org/:_authToken=". If found, it returns the token without the prefix.
// If the token is not found, it returns an error with a message indicating the absence of the token.
func getNpmToken() (string, error) {
	npmrcPath := filepath.Join(os.Getenv("HOME"), ".npmrc")
	content, err := os.ReadFile(npmrcPath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "//registry.npmjs.org/:_authToken=") {
			return strings.TrimPrefix(line, "//registry.npmjs.org/:_authToken="), nil
		}
	}
	return "", fmt.Errorf("auth token not found in %s", npmrcPath)
}

// fetchOrgPackages fetches the list of packages for an organization
func fetchOrgPackages(orgName, token string) (map[string]interface{}, error) {
	apiURL := fmt.Sprintf("https://registry.npmjs.org/-/org/%s/package", orgName)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for org %s: %w", orgName, err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch packages for org %s: %w", orgName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch packages for org %s (status: %d)", orgName, resp.StatusCode)
	}

	var packages map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&packages); err != nil {
		return nil, fmt.Errorf("failed to decode response for org %s: %w", orgName, err)
	}
	return packages, nil
}

// main is the entry point of the program
func main() {
	if len(os.Args) < 2 {
		log.Fatal("Please provide the organization name as a command-line argument.")
	}
	orgName := os.Args[1]

	token, err := getNpmToken()
	if err != nil {
		log.Fatalf("Failed to retrieve npm token: %v", err)
	}

	packages, err := fetchOrgPackages(orgName, token)
	if err != nil {
		log.Fatalf("Failed to fetch packages: %v", err)
	}

	if len(packages) == 0 {
		log.Fatalf("No packages found for org %s", orgName)
	}

	results := make(chan PackageInfo)
	var wg sync.WaitGroup

	for pkgName := range packages {
		wg.Add(1)
		go fetchPackageSize(pkgName, token, &wg, results)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var packageInfos []PackageInfo
	for pkg := range results {
		packageInfos = append(packageInfos, pkg)
	}

	if len(packageInfos) == 0 {
		log.Fatalf("No package sizes retrieved for org %s", orgName)
	}

	// Sort packages by size from largest to smallest
	sort.Slice(packageInfos, func(i, j int) bool {
		return packageInfos[i].RawSize > packageInfos[j].RawSize
	})

	// Write results to CSV
	csvPath := filepath.Join(".", "package-sizes.csv")
	if err := writeCSV(csvPath, packageInfos); err != nil {
		log.Fatalf("Failed to write CSV: %v", err)
	}
	fmt.Printf("CSV file created: %s\n", csvPath)

	// Print results as a table
	printTable(packageInfos)
}
