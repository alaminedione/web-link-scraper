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
		return nil, fmt.Errorf("URL invalide: %v", err)
	}

	// Configuration SSL et client HTTP avec headers réalistes
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Ignorer les erreurs SSL
		},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   15 * time.Second,
	}

	// Créer le dossier de sortie s'il n'existe pas
	if outputDir != "" {
		err := os.MkdirAll(outputDir, 0755)
		if err != nil {
			return nil, fmt.Errorf("impossible de créer le dossier de sortie: %v", err)
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
	fmt.Printf("❌ ERREUR: %s\n", err)
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

	fmt.Printf("🔍 [Profondeur %d] Scraping: %s\n", depth, targetURL)

	newInternalLinks, err := ls.scrapePage(targetURL, depth)
	if err != nil {
		ls.addError(fmt.Sprintf("Erreur sur %s: %v", targetURL, err))
		return
	}

	// Si on n'a pas atteint la profondeur maximale, continuer avec les liens internes
	if depth < ls.maxDepth {
		// Scraper récursivement les liens internes trouvés sur cette page
		for _, link := range newInternalLinks {
			ls.mutex.RLock()
			alreadyVisited := ls.visitedURL[link]
			ls.mutex.RUnlock()

			if !alreadyVisited { // Pas besoin de vérifier isInternalLink ici, car newInternalLinks ne contient que des liens internes
				ls.ScrapeLinksRecursive(link, depth+1)
			}
		}
	}
}

func (ls *LinkScraper) scrapePage(targetURL string, depth int) ([]string, error) {
	// Créer la requête avec des headers réalistes
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création de la requête: %v", err)
	}

	// Headers réalistes pour éviter les blocages
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Cache-Control", "max-age=0")

	// Faire la requête HTTP
	resp, err := ls.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la requête: %v", err)
	}
	defer resp.Body.Close()

	var reader io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de la création du lecteur gzip: %v", err)
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
		return nil, fmt.Errorf("code de statut HTTP: %d", resp.StatusCode)
	}

	// Vérifier le Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/html") {
		return nil, fmt.Errorf("contenu non-HTML détecté: %s", contentType)
	}

	// Parser le HTML
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("erreur lors du parsing HTML: %v", err)
	}

	fmt.Printf("✅ Page chargée avec succès: %s\n", targetURL)

	// Sauvegarder le contenu HTML pour inspection
	// htmlContent, _ := doc.Html()
	// tempFileName := fmt.Sprintf("temp_html_%s.html", strings.ReplaceAll(ls.baseURL.Host, ".", "_"))
	// os.WriteFile(tempFileName, []byte(htmlContent), 0644)
	// fmt.Printf("📝 Contenu HTML sauvegardé dans: %s pour débogage\n", tempFileName)

	// Extraire tous les liens <a href="">
	linkCount := 0
	newInternalLinks := []string{}

	// Extraire tous les liens <a href="">
	// foundALinks := 0
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		// foundALinks++
		href, exists := s.Attr("href")
		if !exists {
			// fmt.Printf("⚠️  Lien <a> sans href trouvé\n")
			return
		}

		// Nettoyer et normaliser l'URL
		cleanURL := ls.normalizeURL(href, targetURL)
		if cleanURL != "" {
			ls.addLink(cleanURL)
			linkCount++
			if ls.isInternalLink(cleanURL) {
				newInternalLinks = append(newInternalLinks, cleanURL)
			}
		}
	})

	// Extraire aussi les liens dans d'autres éléments si nécessaire
	// fmt.Printf("🔍 Recherche de liens <a>...\n")
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		// foundALinks++
		href, exists := s.Attr("href")
		if !exists {
			// fmt.Printf("⚠️  Lien <a> sans href trouvé\n")
			return
		}

		// Nettoyer et normaliser l'URL
		cleanURL := ls.normalizeURL(href, targetURL)
		if cleanURL != "" {
			ls.addLink(cleanURL)
			linkCount++
			if ls.isInternalLink(cleanURL) {
				newInternalLinks = append(newInternalLinks, cleanURL)
			}
		}
	})

	// fmt.Printf("📊 %d liens <a> trouvés sur cette page\n", foundALinks)

	// Extraire aussi les liens dans d'autres éléments si nécessaire
	// foundLinkElements := 0
	// fmt.Printf("🔍 Recherche de liens <link>...\n")
	doc.Find("link[href]").Each(func(i int, s *goquery.Selection) {
		// foundLinkElements++
		href, exists := s.Attr("href")
		if !exists {
			// fmt.Printf("⚠️  Élément <link> sans href trouvé\n")
			return
		}

		rel, _ := s.Attr("rel")
		// Ne garder que certains types de liens
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
	// fmt.Printf("📊 %d éléments <link> trouvés sur cette page\n", foundLinkElements)

	// fmt.Printf("📊 Total de %d liens ajoutés sur cette page\n", linkCount)
	return newInternalLinks, nil
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

	// Classer le lien
	if ls.isInternalLink(link) {
		ls.internalLinks = append(ls.internalLinks, link)
		// fmt.Printf("🔗 Ajouté lien interne: %s\n", link)
	} else {
		ls.externalLinks = append(ls.externalLinks, link)
		// fmt.Printf("🔗 Ajouté lien externe: %s\n", link)
	}
}

func (ls *LinkScraper) isInternalLink(link string) bool {
	parsedLink, err := url.Parse(link)
	if err != nil {
		return false
	}

	// Si pas de host, c'est un lien relatif donc interne
	if parsedLink.Host == "" {
		return true
	}

	// Comparer les domaines (avec et sans www)
	baseHost := strings.ToLower(ls.baseURL.Host)
	linkHost := strings.ToLower(parsedLink.Host)

	// Supprimer www. pour la comparaison
	baseHost = strings.TrimPrefix(baseHost, "www.")
	linkHost = strings.TrimPrefix(linkHost, "www.")

	return baseHost == linkHost
}

func (ls *LinkScraper) normalizeURL(href, baseURL string) string {
	// Nettoyer l'href
	href = strings.TrimSpace(href)

	// Ignorer les liens vides, les ancres, javascript et mailto
	if href == "" || strings.HasPrefix(href, "#") ||
		strings.HasPrefix(href, "javascript:") ||
		strings.HasPrefix(href, "mailto:") ||
		strings.HasPrefix(href, "tel:") ||
		strings.HasPrefix(href, "ftp:") ||
		strings.HasPrefix(href, "file:") {
		return ""
	}

	// Parser l'URL de base
	base, err := url.Parse(baseURL)
	if err != nil {
		fmt.Printf("⚠️  Erreur parsing URL de base %s: %v\n", baseURL, err)
		return ""
	}

	// Parser le lien href
	link, err := url.Parse(href)
	if err != nil {
		fmt.Printf("⚠️  Erreur parsing href %s: %v\n", href, err)
		return ""
	}

	// Résoudre l'URL relative par rapport à la base
	resolved := base.ResolveReference(link)

	// Nettoyer l'URL (supprimer les fragments et paramètres inutiles)
	resolved.Fragment = ""

	// Supprimer les paramètres de tracking courants
	query := resolved.Query()
	trackingParams := []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content", "fbclid", "gclid"}
	for _, param := range trackingParams {
		query.Del(param)
	}
	resolved.RawQuery = query.Encode()

	finalURL := resolved.String()

	// Debug pour voir les transformations
	// Debug pour voir les transformations
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

	// Créer le nom de fichier avec timestamp
	domain := strings.ReplaceAll(ls.baseURL.Host, ".", "_")
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("scraping_%s_%s.json", domain, timestamp)
	filepath := filepath.Join(ls.outputDir, filename)

	// Sauvegarder en JSON
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("erreur lors de l'encodage JSON: %v", err)
	}

	err = os.WriteFile(filepath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("erreur lors de l'écriture du fichier: %v", err)
	}

	fmt.Printf("💾 Résultats sauvegardés dans: %s\n", filepath)
	return nil
}

func (ls *LinkScraper) PrintDetailedStats() {
	results := ls.GetResults()

	fmt.Printf("\n" + strings.Repeat("=", 50) + "\n")
	fmt.Printf("📊 STATISTIQUES DÉTAILLÉES\n")
	fmt.Printf(strings.Repeat("=", 50) + "\n")
	fmt.Printf("🌐 Site web: %s\n", results.BaseURL)
	fmt.Printf("⏱️  Temps d'exécution: %s\n", results.Statistics.ExecutionTime)
	fmt.Printf("📄 Pages visitées: %d\n", results.Statistics.PagesVisited)
	fmt.Printf("🔗 Total des liens: %d\n", results.Statistics.TotalLinks)
	fmt.Printf("🏠 Liens internes: %d\n", results.Statistics.InternalCount)
	fmt.Printf("🌍 Liens externes: %d\n", results.Statistics.ExternalCount)
	fmt.Printf("📊 Profondeur max atteinte: %d\n", results.Statistics.MaxDepthReached)
	fmt.Printf("❌ Erreurs rencontrées: %d\n", results.Statistics.ErrorsCount)

	if len(results.Errors) > 0 {
		fmt.Printf("\n🚨 ERREURS:\n")
		for _, err := range results.Errors {
			fmt.Printf("   • %s\n", err)
		}
	}

	fmt.Printf(strings.Repeat("=", 50) + "\n")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run get-links <URL> [profondeur_max] [dossier_sortie]")
		fmt.Println("Exemple: go run get-links https://example.com 2 ./results")
		fmt.Println("Paramètres:")
		fmt.Println("  URL: L'URL du site à scraper")
		fmt.Println("  profondeur_max: Profondeur maximale du scraping récursif (défaut: 1)")
		fmt.Println("  dossier_sortie: Dossier pour sauvegarder les résultats (défaut: ./scraping_results)")
		os.Exit(1)
	}

	targetURL := os.Args[1]
	maxDepth := 1
	outputDir := "./scraping_results"

	// Parser la profondeur maximale si fournie
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &maxDepth)
	}

	// Parser le dossier de sortie si fourni
	if len(os.Args) > 3 {
		outputDir = os.Args[3]
	}

	fmt.Printf("🚀 Démarrage du scraping de: %s\n", targetURL)
	fmt.Printf("📊 Profondeur maximale: %d\n", maxDepth)
	fmt.Printf("💾 Dossier de sortie: %s\n", outputDir)
	fmt.Println(strings.Repeat("-", 50))

	// Créer le scraper
	scraper, err := NewLinkScraper(targetURL, maxDepth, outputDir)
	if err != nil {
		log.Fatalf("❌ Erreur lors de la création du scraper: %v", err)
	}

	// Test de connexion initial
	fmt.Printf("🔗 Test de connexion à %s...\n", targetURL)
	resp, err := http.Head(targetURL)
	if err == nil {
		resp.Body.Close()
		fmt.Printf("✅ Connexion réussie (Status: %d)\n", resp.StatusCode)
	} else {
		fmt.Printf("⚠️  Test de connexion échoué, mais on continue: %v\n", err)
	}

	// Lancer le scraping récursif
	scraper.ScrapeLinksRecursive(targetURL, 0)

	// Sauvegarder les résultats
	err = scraper.SaveResults()
	if err != nil {
		fmt.Printf("⚠️  Erreur lors de la sauvegarde: %v\n", err)
	}

	// Afficher les statistiques détaillées
	scraper.PrintDetailedStats()

	fmt.Printf("\n✅ Scraping terminé avec succès!\n")
}
