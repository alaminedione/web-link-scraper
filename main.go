package main

import (
	"compress/flate"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type LinkScraper struct {
	baseURL       *url.URL
	client        *http.Client
	visitedURL    map[string]bool
	links         []string
	internalLinks []string
	externalLinks []string
	errors        []string
	mutex         sync.RWMutex
	maxDepth      int
	currentDepth  int
	startTime     time.Time
	outputDir     string
}

type ScrapingResults struct {
	BaseURL       string        `json:"base_url"`
	TotalLinks    int           `json:"total_links"`
	InternalLinks []string      `json:"internal_links"`
	ExternalLinks []string      `json:"external_links"`
	AllLinks      []string      `json:"all_links"`
	Errors        []string      `json:"errors"`
	Statistics    ScrapingStats `json:"statistics"`
	Timestamp     string        `json:"timestamp"`
}

type ScrapingStats struct {
	PagesVisited    int    `json:"pages_visited"`
	TotalLinks      int    `json:"total_links"`
	InternalCount   int    `json:"internal_count"`
	ExternalCount   int    `json:"external_count"`
	ErrorsCount     int    `json:"errors_count"`
	ExecutionTime   string `json:"execution_time"`
	MaxDepthReached int    `json:"max_depth_reached"`
}

func NewLinkScraper(baseURL string, maxDepth int, outputDir string) (*LinkScraper, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}

	// SSL configuration and HTTP client with realistic headers
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Ignore SSL errors
		},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   15 * time.Second,
	}

	// Create output directory if it doesn't exist
	if outputDir != "" {
		err := os.MkdirAll(outputDir, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to create output directory: %v", err)
		}
	}

	return &LinkScraper{
		baseURL:       parsedURL,
		client:        client,
		visitedURL:    make(map[string]bool),
		links:         make([]string, 0),
		internalLinks: make([]string, 0),
		externalLinks: make([]string, 0),
		errors:        make([]string, 0),
		maxDepth:      maxDepth,
		currentDepth:  0,
		startTime:     time.Now(),
		outputDir:     outputDir,
	}, nil
}

func (ls *LinkScraper) addError(err string) {
	ls.mutex.Lock()
	defer ls.mutex.Unlock()
	ls.errors = append(ls.errors, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), err))
	fmt.Printf("❌ ERROR: %s\n", err)
}

func (ls *LinkScraper) ScrapeLinksRecursive(targetURL string, depth int) {
	if depth > ls.maxDepth {
		return
	}

	ls.mutex.RLock()
	visited := ls.visitedURL[targetURL]
	ls.mutex.RUnlock()

	if visited {
		return
	}

	ls.mutex.Lock()
	ls.visitedURL[targetURL] = true
	if depth > ls.currentDepth {
		ls.currentDepth = depth
	}
	ls.mutex.Unlock()

	fmt.Printf("🔍 [Depth %d] Scraping: %s\n", depth, targetURL)

	newInternalLinks, err := ls.scrapePage(targetURL, depth)
	if err != nil {
		ls.addError(fmt.Sprintf("Error on %s: %v", targetURL, err))
		return
	}

	// If max depth not reached, continue with internal links
	if depth < ls.maxDepth {
		// Recursively scrape internal links found on this page
		for _, link := range newInternalLinks {
			ls.mutex.RLock()
			alreadyVisited := ls.visitedURL[link]
			ls.mutex.RUnlock()

			if !alreadyVisited { // No need to check isInternalLink here, as newInternalLinks only contains internal links
				ls.ScrapeLinksRecursive(link, depth+1)
			}
		}
	}
}

	func (ls *LinkScraper) scrapePage(targetURL string, depth int) ([]string, error) {
	// Create request with realistic headers
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Realistic headers to avoid blocking
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,fr;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Cache-Control", "max-age=0")

	// Make HTTP request
	resp, err := ls.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	var reader io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error creating gzip reader: %v", err)
		}
		defer gzReader.Close()
		reader = gzReader
	case "deflate":
		flReader := flate.NewReader(resp.Body)
		defer flReader.Close()
		reader = flReader
	default:
		reader = resp.Body
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP status code: %d", resp.StatusCode)
	}

	// Check Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/html") {
		return nil, fmt.Errorf("non-HTML content detected: %s", contentType)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	fmt.Printf("✅ Page loaded successfully: %s\n", targetURL)

	// Save HTML content for inspection
	// htmlContent, _ := doc.Html()
	// tempFileName := fmt.Sprintf("temp_html_%s.html", strings.ReplaceAll(ls.baseURL.Host, ".", "_"))
	// os.WriteFile(tempFileName, []byte(htmlContent), 0644)
	// fmt.Printf("📝 HTML content saved to: %s for debugging\n", tempFileName)

	// Extract all <a href=""> links
	linkCount := 0
	newInternalLinks := []string{}

	// Extract all <a href=""> links
	// foundALinks := 0
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		// foundALinks++
		href, exists := s.Attr("href")
		if !exists {
			// fmt.Printf("⚠️  <a> link without href found\n")
			return
		}

		// Clean and normalize URL
		cleanURL := ls.normalizeURL(href, targetURL)
		if cleanURL != "" {
			ls.addLink(cleanURL)
			linkCount++
			if ls.isInternalLink(cleanURL) {
				newInternalLinks = append(newInternalLinks, cleanURL)
			}
		}
	})

	// Also extract links from other elements if necessary
	// fmt.Printf("🔍 Searching for <a> links...\n")
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		// foundALinks++
		href, exists := s.Attr("href")
		if !exists {
			// fmt.Printf("⚠️  <a> link without href found\n")
			return
		}

		// Clean and normalize URL
		cleanURL := ls.normalizeURL(href, targetURL)
		if cleanURL != "" {
			ls.addLink(cleanURL)
			linkCount++
			if ls.isInternalLink(cleanURL) {
				newInternalLinks = append(newInternalLinks, cleanURL)
			}
		}
	})

	// fmt.Printf("📊 %d <a> links found on this page\n", foundALinks)

	// Also extract links from other elements if necessary
	// foundLinkElements := 0
	// fmt.Printf("🔍 Searching for <link> elements...\n")
	doc.Find("link[href]").Each(func(i int, s *goquery.Selection) {
		// foundLinkElements++
		href, exists := s.Attr("href")
		if !exists {
			// fmt.Printf("⚠️  <link> element without href found\n")
			return
		}

		rel, _ := s.Attr("rel")
		// Only keep certain types of links
		if strings.Contains(rel, "canonical") || strings.Contains(rel, "alternate") {
			cleanURL := ls.normalizeURL(href, targetURL)
			if cleanURL != "" {
				ls.addLink(cleanURL)
				linkCount++
				if ls.isInternalLink(cleanURL) {
					newInternalLinks = append(newInternalLinks, cleanURL)
				}
			}
		}
	})
	// fmt.Printf("📊 %d <link> elements found on this page\n", foundLinkElements)

	// fmt.Printf("📊 Total of %d links added on this page\n", linkCount)
	return newInternalLinks, nil
}

func (ls *LinkScraper) addLink(link string) {
	ls.mutex.Lock()
	defer ls.mutex.Unlock()

	// Avoid duplicates
	for _, existingLink := range ls.links {
		if existingLink == link {
			return
		}
	}

	ls.links = append(ls.links, link)

	// Categorize the link
	if ls.isInternalLink(link) {
		ls.internalLinks = append(ls.internalLinks, link)
		// fmt.Printf("🔗 Added internal link: %s\n", link)
	} else {
		ls.externalLinks = append(ls.externalLinks, link)
		// fmt.Printf("🔗 Added external link: %s\n", link)
	}
}

func (ls *LinkScraper) isInternalLink(link string) bool {
	parsedLink, err := url.Parse(link)
	if err != nil {
		return false
	}

	// If no host, it's a relative link, thus internal
	if parsedLink.Host == "" {
		return true
	}

	// Compare domains (with and without www)
	baseHost := strings.ToLower(ls.baseURL.Host)
	linkHost := strings.ToLower(parsedLink.Host)

	// Remove www. for comparison
	baseHost = strings.TrimPrefix(baseHost, "www.")
	linkHost = strings.TrimPrefix(linkHost, "www.")

	return baseHost == linkHost
}

func (ls *LinkScraper) normalizeURL(href, baseURL string) string {
	// Clean the href
	href = strings.TrimSpace(href)

	// Ignore empty links, anchors, javascript, and mailto
	if href == "" || strings.HasPrefix(href, "#") ||
		strings.HasPrefix(href, "javascript:") ||
		strings.HasPrefix(href, "mailto:") ||
		strings.HasPrefix(href, "tel:") ||
		strings.HasPrefix(href, "ftp:") ||
		strings.HasPrefix(href, "file:") {
		return ""
	}

	// Parse the base URL
	base, err := url.Parse(baseURL)
	if err != nil {
		fmt.Printf("⚠️  Error parsing base URL %s: %v\n", baseURL, err)
		return ""
	}

	// Parse the href link
	link, err := url.Parse(href)
	if err != nil {
		fmt.Printf("⚠️  Error parsing href %s: %v\n", href, err)
		return ""
	}

	// Resolve the relative URL against the base
	resolved := base.ResolveReference(link)

	// Clean the URL (remove fragments and unnecessary parameters)
	resolved.Fragment = ""

	// Remove common tracking parameters
	query := resolved.Query()
	trackingParams := []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content", "fbclid", "gclid"}
	for _, param := range trackingParams {
		query.Del(param)
	}
	resolved.RawQuery = query.Encode()

	finalURL := resolved.String()

	// Debug to see transformations
	// Debug to see transformations
	// if href != finalURL {
	// 	fmt.Printf("🔄 Transformation: %s -> %s\n", href, finalURL)
	// }

	return finalURL
}

func (ls *LinkScraper) GetResults() ScrapingResults {
	ls.mutex.RLock()
	defer ls.mutex.RUnlock()

	return ScrapingResults{
		BaseURL:       ls.baseURL.String(),
		TotalLinks:    len(ls.links),
		InternalLinks: ls.internalLinks,
		ExternalLinks: ls.externalLinks,
		AllLinks:      ls.links,
		Errors:        ls.errors,
		Statistics: ScrapingStats{
			PagesVisited:    len(ls.visitedURL),
			TotalLinks:      len(ls.links),
			InternalCount:   len(ls.internalLinks),
			ExternalCount:   len(ls.externalLinks),
			ErrorsCount:     len(ls.errors),
			ExecutionTime:   time.Since(ls.startTime).String(),
			MaxDepthReached: ls.currentDepth,
		},
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}
}

func (ls *LinkScraper) SaveResults() error {
	if ls.outputDir == "" {
		return nil
	}

	results := ls.GetResults()

	// Create filename with timestamp
	domain := strings.ReplaceAll(ls.baseURL.Host, ".", "_")
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("scraping_%s_%s.json", domain, timestamp)
	filepath := filepath.Join(ls.outputDir, filename)

	// Save as JSON
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("error encoding JSON: %v", err)
	}

	err = os.WriteFile(filepath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}

	fmt.Printf("💾 Results saved to: %s\n", filepath)
	return nil
}

func (ls *LinkScraper) PrintDetailedStats() {
	results := ls.GetResults()

	fmt.Printf("\n" + strings.Repeat("=", 50) + "\n")
	fmt.Printf("📊 DETAILED STATISTICS\n")
	fmt.Printf(strings.Repeat("=", 50) + "\n")
	fmt.Printf("🌐 Website: %s\n", results.BaseURL)
	fmt.Printf("⏱️  Execution Time: %s\n", results.Statistics.ExecutionTime)
	fmt.Printf("📄 Pages Visited: %d\n", results.Statistics.PagesVisited)
	fmt.Printf("🔗 Total Links: %d\n", results.Statistics.TotalLinks)
	fmt.Printf("🏠 Internal Links: %d\n", results.Statistics.InternalCount)
	fmt.Printf("🌍 External Links: %d\n", results.Statistics.ExternalCount)
	fmt.Printf("📊 Max Depth Reached: %d\n", results.Statistics.MaxDepthReached)
	fmt.Printf("❌ Errors Encountered: %d\n", results.Statistics.ErrorsCount)

	if len(results.Errors) > 0 {
		fmt.Printf("\n🚨 ERRORS:\n")
		for _, err := range results.Errors {
			fmt.Printf("   • %s\n", err)
		}
	}

	fmt.Printf(strings.Repeat("=", 50) + "\n")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run get-links <URL> [max_depth] [output_folder]")
		fmt.Println("Example: go run get-links https://example.com 2 ./results")
		fmt.Println("Parameters:")
		fmt.Println("  URL: The URL of the website to scrape")
		fmt.Println("  max_depth: Maximum depth for recursive scraping (default: 1)")
		fmt.Println("  output_folder: Folder to save results (default: ./scraping_results)")
		os.Exit(1)
	}

	targetURL := os.Args[1]
	maxDepth := 1
	outputDir := "./scraping_results"

	// Parse max depth if provided
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &maxDepth)
	}

	// Parse output directory if provided
	if len(os.Args) > 3 {
		outputDir = os.Args[3]
	}

	fmt.Printf("🚀 Starting ultra-fast scraping of: %s\n", targetURL)
	fmt.Printf("📊 Maximum depth: %d\n", maxDepth)
	fmt.Printf("💾 Output directory: %s\n", outputDir)
	fmt.Println(strings.Repeat("-", 50))

	// Create the scraper
	scraper, err := NewLinkScraper(targetURL, maxDepth, outputDir)
	if err != nil {
		log.Fatalf("❌ Error creating scraper: %v", err)
	}

	// Initial connection test
	fmt.Printf("🔗 Testing connection to %s...\n", targetURL)
	resp, err := http.Head(targetURL)
	if err == nil {
		resp.Body.Close()
		fmt.Printf("✅ Connection successful (Status: %d)\n", resp.StatusCode)
	} else {
		fmt.Printf("⚠️  Connection test failed, but continuing: %v\n", err)
	}

	// Start recursive scraping
	scraper.ScrapeLinksRecursive(targetURL, 0)

	// Save results
	err = scraper.SaveResults()
	if err != nil {
		fmt.Printf("⚠️  Error saving results: %v\n", err)
	}

	// Print detailed statistics
	scraper.PrintDetailedStats()

	fmt.Printf("\n✅ Scraping completed successfully!\n")
}
