package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type LinkScraper struct {
	baseURL    *url.URL
	client     *http.Client
	visitedURL map[string]bool
	links      []string
}

func NewLinkScraper(baseURL string) (*LinkScraper, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("URL invalide: %v", err)
	}

	return &LinkScraper{
		baseURL: parsedURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		visitedURL: make(map[string]bool),
		links:      make([]string, 0),
	}, nil
}

func (ls *LinkScraper) ScrapeLinks(targetURL string) error {
	// Éviter les doublons
	if ls.visitedURL[targetURL] {
		return nil
	}
	ls.visitedURL[targetURL] = true

	fmt.Printf("Scraping: %s\n", targetURL)

	// Faire la requête HTTP
	resp, err := ls.client.Get(targetURL)
	if err != nil {
		return fmt.Errorf("erreur lors de la requête: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("code de statut: %d", resp.StatusCode)
	}

	// Parser le HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("erreur lors du parsing HTML: %v", err)
	}

	// Extraire tous les liens
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		// Nettoyer et normaliser l'URL
		cleanURL := ls.normalizeURL(href, targetURL)
		if cleanURL != "" && !ls.containsLink(cleanURL) {
			ls.links = append(ls.links, cleanURL)
		}
	})

	return nil
}

func (ls *LinkScraper) normalizeURL(href, baseURL string) string {
	// Ignorer les liens vides, les ancres et javascript
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
		return ""
	}

	// Parser l'URL de base
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	// Parser le lien href
	link, err := url.Parse(href)
	if err != nil {
		return ""
	}

	// Résoudre l'URL relative par rapport à la base
	resolved := base.ResolveReference(link)

	// Nettoyer l'URL (supprimer les fragments)
	resolved.Fragment = ""

	return resolved.String()
}

func (ls *LinkScraper) containsLink(link string) bool {
	for _, existingLink := range ls.links {
		if existingLink == link {
			return true
		}
	}
	return false
}

func (ls *LinkScraper) GetLinks() []string {
	return ls.links
}

func (ls *LinkScraper) GetInternalLinks() []string {
	var internalLinks []string
	for _, link := range ls.links {
		parsedLink, err := url.Parse(link)
		if err != nil {
			continue
		}

		// Vérifier si le lien appartient au même domaine
		if parsedLink.Host == ls.baseURL.Host || parsedLink.Host == "" {
			internalLinks = append(internalLinks, link)
		}
	}
	return internalLinks
}

func (ls *LinkScraper) GetExternalLinks() []string {
	var externalLinks []string
	for _, link := range ls.links {
		parsedLink, err := url.Parse(link)
		if err != nil {
			continue
		}

		// Vérifier si le lien est externe
		if parsedLink.Host != ls.baseURL.Host && parsedLink.Host != "" {
			externalLinks = append(externalLinks, link)
		}
	}
	return externalLinks
}

func (ls *LinkScraper) PrintStats() {
	fmt.Printf("\n=== STATISTIQUES ===\n")
	fmt.Printf("Total des liens trouvés: %d\n", len(ls.links))
	fmt.Printf("Liens internes: %d\n", len(ls.GetInternalLinks()))
	fmt.Printf("Liens externes: %d\n", len(ls.GetExternalLinks()))
	fmt.Printf("Pages visitées: %d\n", len(ls.visitedURL))
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run scraper.go <URL>")
		fmt.Println("Exemple: go run scraper.go https://example.com")
		os.Exit(1)
	}

	targetURL := os.Args[1]

	// Créer le scraper
	scraper, err := NewLinkScraper(targetURL)
	if err != nil {
		log.Fatalf("Erreur lors de la création du scraper: %v", err)
	}

	// Scraper la page principale
	err = scraper.ScrapeLinks(targetURL)
	if err != nil {
		log.Fatalf("Erreur lors du scraping: %v", err)
	}

	// Afficher tous les liens
	fmt.Println("\n=== TOUS LES LIENS ===")
	for i, link := range scraper.GetLinks() {
		fmt.Printf("%d. %s\n", i+1, link)
	}

	// Afficher les liens internes
	internalLinks := scraper.GetInternalLinks()
	if len(internalLinks) > 0 {
		fmt.Println("\n=== LIENS INTERNES ===")
		for i, link := range internalLinks {
			fmt.Printf("%d. %s\n", i+1, link)
		}
	}

	// Afficher les liens externes
	externalLinks := scraper.GetExternalLinks()
	if len(externalLinks) > 0 {
		fmt.Println("\n=== LIENS EXTERNES ===")
		for i, link := range externalLinks {
			fmt.Printf("%d. %s\n", i+1, link)
		}
	}

	// Afficher les statistiques
	scraper.PrintStats()
}
