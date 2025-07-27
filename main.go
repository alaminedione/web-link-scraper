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

// Ajout de structures pour la classification
type LinkCategory string

const (
	CategoryHTML       LinkCategory = "html_pages"
	CategoryDocument   LinkCategory = "documents"
	CategoryImage      LinkCategory = "images"
	CategoryScript     LinkCategory = "scripts"
	CategoryStylesheet LinkCategory = "stylesheets"
	CategoryMultimedia LinkCategory = "multimedia"
	CategoryArchive    LinkCategory = "archives"
	CategoryOther      LinkCategory = "other"
)

type ClassifiedLink struct {
	URL      string       `json:"url"`
	Category LinkCategory `json:"category"`
	FileType string       `json:"file_type"`
}

type LinkScraper struct {
	baseURL         *url.URL
	client          *http.Client
	visitedURL      map[string]bool
	links           []string
	internalLinks   []string
	externalLinks   []string
	classifiedLinks map[LinkCategory][]ClassifiedLink // Nouvelle structure pour la classification
	errors          []string
	mutex           sync.RWMutex
	maxDepth        int
	currentDepth    int
	startTime       time.Time
	outputDir       string
}

type ScrapingResults struct {
	BaseURL          string                             `json:"base_url"`
	TotalLinks       int                                `json:"total_links"`
	InternalLinks    []string                           `json:"internal_links"`
	ExternalLinks    []string                           `json:"external_links"`
	AllLinks         []string                           `json:"all_links"`
	ClassifiedLinks  map[LinkCategory][]ClassifiedLink `json:"classified_links"`
	CategorySummary  map[LinkCategory]int               `json:"category_summary"`
	Errors           []string                           `json:"errors"`
	Statistics       ScrapingStats                      `json:"statistics"`
	Timestamp        string                             `json:"timestamp"`
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

// Définition des extensions par catégorie
var fileExtensions = map[LinkCategory][]string{
	CategoryHTML:       {".html", ".htm", ".xhtml", ".php", ".asp", ".aspx", ".jsp", ".do"},
	CategoryDocument:   {".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".odt", ".ods", ".odp", ".txt", ".rtf", ".csv"},
	CategoryImage:      {".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp", ".ico", ".tiff", ".tif"},
	CategoryScript:     {".js", ".mjs", ".ts"},
	CategoryStylesheet: {".css", ".scss", ".sass", ".less"},
	CategoryMultimedia: {".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".mp3", ".wav", ".ogg", ".m4a", ".flac"},
	CategoryArchive:    {".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz"},
}

func NewLinkScraper(baseURL string, maxDepth int, outputDir string) (*LinkScraper, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   15 * time.Second,
	}

	if outputDir != "" {
		err := os.MkdirAll(outputDir, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to create output directory: %v", err)
		}
	}

	// Initialisation de la map pour les liens classifiés
	classifiedLinks := make(map[LinkCategory][]ClassifiedLink)
	for category := range fileExtensions {
		classifiedLinks[category] = make([]ClassifiedLink, 0)
	}
	classifiedLinks[CategoryOther] = make([]ClassifiedLink, 0)

	return &LinkScraper{
		baseURL:         parsedURL,
		client:          client,
		visitedURL:      make(map[string]bool),
		links:           make([]string, 0),
		internalLinks:   make([]string, 0),
		externalLinks:   make([]string, 0),
		classifiedLinks: classifiedLinks,
		errors:          make([]string, 0),
		maxDepth:        maxDepth,
		currentDepth:    0,
		startTime:       time.Now(),
		outputDir:       outputDir,
	}, nil
}

// Nouvelle fonction pour classifier un lien
func (ls *LinkScraper) classifyLink(link string) (LinkCategory, string) {
	parsedURL, err := url.Parse(link)
	if err != nil {
		return CategoryOther, "unknown"
	}

	path := strings.ToLower(parsedURL.Path)
	
	// Si pas d'extension, vérifier si c'est probablement une page HTML
	if !strings.Contains(path, ".") || strings.HasSuffix(path, "/") {
		return CategoryHTML, "html"
	}

	// Extraire l'extension
	ext := filepath.Ext(path)
	if ext == "" {
		return CategoryHTML, "html"
	}

	// Chercher dans nos catégories
	for category, extensions := range fileExtensions {
		for _, fileExt := range extensions {
			if ext == fileExt {
				return category, strings.TrimPrefix(ext, ".")
			}
		}
	}

	return CategoryOther, strings.TrimPrefix(ext, ".")
}

func (ls *LinkScraper) addLink(link string) {
	ls.mutex.Lock()
	defer ls.mutex.Unlock()

	// Éviter les doublons
	for _, existingLink := range ls.links {
		if existingLink == link {
			return
		}
	}

	ls.links = append(ls.links, link)

	// Classifier le lien
	category, fileType := ls.classifyLink(link)
	classifiedLink := ClassifiedLink{
		URL:      link,
		Category: category,
		FileType: fileType,
	}
	ls.classifiedLinks[category] = append(ls.classifiedLinks[category], classifiedLink)

	// Catégoriser comme interne ou externe
	if ls.isInternalLink(link) {
		ls.internalLinks = append(ls.internalLinks, link)
	} else {
		ls.externalLinks = append(ls.externalLinks, link)
	}
}

func (ls *LinkScraper) GetResults() ScrapingResults {
	ls.mutex.RLock()
	defer ls.mutex.RUnlock()

	// Créer un résumé par catégorie
	categorySummary := make(map[LinkCategory]int)
	for category, links := range ls.classifiedLinks {
		categorySummary[category] = len(links)
	}

	return ScrapingResults{
		BaseURL:         ls.baseURL.String(),
		TotalLinks:      len(ls.links),
		InternalLinks:   ls.internalLinks,
		ExternalLinks:   ls.externalLinks,
		AllLinks:        ls.links,
		ClassifiedLinks: ls.classifiedLinks,
		CategorySummary: categorySummary,
		Errors:          ls.errors,
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

	// Afficher le résumé par catégorie
	fmt.Printf("\n📂 LINKS BY CATEGORY:\n")
	categoryIcons := map[LinkCategory]string{
		CategoryHTML:       "📄",
		CategoryDocument:   "📑",
		CategoryImage:      "🖼️",
		CategoryScript:     "⚙️",
		CategoryStylesheet: "🎨",
		CategoryMultimedia: "🎬",
		CategoryArchive:    "📦",
		CategoryOther:      "❓",
	}

	for category, count := range results.CategorySummary {
		if count > 0 {
			icon := categoryIcons[category]
			fmt.Printf("   %s %s: %d\n", icon, strings.Title(string(category)), count)
		}
	}

	// Afficher quelques exemples par catégorie
	fmt.Printf("\n📋 SAMPLE LINKS BY CATEGORY:\n")
	for category, links := range results.ClassifiedLinks {
		if len(links) > 0 {
			icon := categoryIcons[category]
			fmt.Printf("\n%s %s (%d total):\n", icon, strings.Title(string(category)), len(links))
			// Afficher max 3 exemples par catégorie
			maxExamples := 3
			if len(links) < maxExamples {
				maxExamples = len(links)
			}
			for i := 0; i < maxExamples; i++ {
				fmt.Printf("   • [%s] %s\n", links[i].FileType, links[i].URL)
			}
			if len(links) > 3 {
				fmt.Printf("   ... and %d more\n", len(links)-3)
			}
		}
	}

	if len(results.Errors) > 0 {
		fmt.Printf("\n🚨 ERRORS:\n")
		for _, err := range results.Errors {
			fmt.Printf("   • %s\n", err)
		}
	}

	fmt.Printf(strings.Repeat("=", 50) + "\n")
}

// Ajouter une fonction pour sauvegarder les résultats classifiés dans des fichiers séparés
func (ls *LinkScraper) SaveClassifiedResults() error {
	if ls.outputDir == "" {
		return nil
	}

	results := ls.GetResults()
	domain := strings.ReplaceAll(ls.baseURL.Host, ".", "_")
	timestamp := time.Now().Format("20060102_150405")

	// Créer un sous-dossier pour cette session
	sessionDir := filepath.Join(ls.outputDir, fmt.Sprintf("%s_%s", domain, timestamp))
	err := os.MkdirAll(sessionDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating session directory: %v", err)
	}

	// Sauvegarder le résumé principal
	mainFile := filepath.Join(sessionDir, "summary.json")
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("error encoding JSON: %v", err)
	}
	err = os.WriteFile(mainFile, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("error writing summary file: %v", err)
	}

	// Sauvegarder chaque catégorie dans un fichier séparé
	for category, links := range results.ClassifiedLinks {
		if len(links) > 0 {
			categoryFile := filepath.Join(sessionDir, fmt.Sprintf("%s.json", category))
			categoryData, err := json.MarshalIndent(links, "", "  ")
			if err != nil {
				continue
			}
			os.WriteFile(categoryFile, categoryData, 0644)
		}
	}

	fmt.Printf("💾 Results saved to: %s\n", sessionDir)
	return nil
}

// Les autres fonctions restent identiques (addError, ScrapeLinksRecursive, scrapePage, etc.)
// Je n'ai modifié que les parties concernant la classification

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

	if depth < ls.maxDepth {
		for _, link := range newInternalLinks {
			ls.mutex.RLock()
			alreadyVisited := ls.visitedURL[link]
			ls.mutex.RUnlock()

			if !alreadyVisited {
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

	// Extract all links
	linkCount := 0
	newInternalLinks := []string{}

	// Extract <a href=""> links
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		// Clean and normalize URL
		cleanURL := ls.normalizeURL(href, targetURL)
		if cleanURL != "" {
			ls.addLink(cleanURL)
			linkCount++
			
			// Only add HTML pages to internal links for recursive scraping
			category, _ := ls.classifyLink(cleanURL)
			if ls.isInternalLink(cleanURL) && category == CategoryHTML {
				newInternalLinks = append(newInternalLinks, cleanURL)
			}
		}
	})

	// Extract <link> elements
	doc.Find("link[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		rel, _ := s.Attr("rel")
		// Only keep certain types of links
		if strings.Contains(rel, "canonical") || strings.Contains(rel, "alternate") {
			cleanURL := ls.normalizeURL(href, targetURL)
			if cleanURL != "" {
				ls.addLink(cleanURL)
				linkCount++
				
				category, _ := ls.classifyLink(cleanURL)
				if ls.isInternalLink(cleanURL) && category == CategoryHTML {
					newInternalLinks = append(newInternalLinks, cleanURL)
				}
			}
		}
	})

	// Extract images
	doc.Find("img[src]").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists {
			return
		}

		cleanURL := ls.normalizeURL(src, targetURL)
		if cleanURL != "" {
			ls.addLink(cleanURL)
			linkCount++
		}
	})

	// Extract scripts
	doc.Find("script[src]").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists {
			return
		}

		cleanURL := ls.normalizeURL(src, targetURL)
		if cleanURL != "" {
			ls.addLink(cleanURL)
			linkCount++
		}
	})

	// Extract stylesheets from link tags
	doc.Find("link[rel='stylesheet']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		cleanURL := ls.normalizeURL(href, targetURL)
		if cleanURL != "" {
			ls.addLink(cleanURL)
			linkCount++
		}
	})

	// Extract video and audio sources
	doc.Find("video source[src], audio source[src]").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists {
			return
		}

		cleanURL := ls.normalizeURL(src, targetURL)
		if cleanURL != "" {
			ls.addLink(cleanURL)
			linkCount++
		}
	})

	// Extract iframe sources
	doc.Find("iframe[src]").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists {
			return
		}

		cleanURL := ls.normalizeURL(src, targetURL)
		if cleanURL != "" {
			ls.addLink(cleanURL)
			linkCount++
		}
	})

	fmt.Printf("📊 Total of %d links found on this page\n", linkCount)
	return newInternalLinks, nil
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
		strings.HasPrefix(href, "file:") ||
		strings.HasPrefix(href, "data:") {
		return ""
	}

	// Parse the base URL
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	// Parse the href link
	link, err := url.Parse(href)
	if err != nil {
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

	return resolved.String()
}

func (ls *LinkScraper) SaveResults() error {
	return ls.SaveClassifiedResults()
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
