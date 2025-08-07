# 🔗 Web Link Scraper

Un outil puissant et tres rapide pour extraire et classifier tous les liens d'un site web avec une analyse récursive en profondeur.

## 📋 Table des matières

- [Fonctionnalités](#-fonctionnalités)
- [Installation](#-installation)
- [Utilisation](#-utilisation)
- [Classification des liens](#-classification-des-liens)
- [Structure des résultats](#-structure-des-résultats)
- [Exemples](#-exemples-de-sortie-console)
- [Configuration avancée](#-configuration-avancée)
- [Dépannage](#-dépannage)
- [Contribution](#-contribution)
- [Licence](#-licence)

## ✨ Fonctionnalités

- 🚀 **Scraping récursif** : Exploration en profondeur des sites web
- 📂 **Classification automatique** : Organisation des liens par type (HTML, documents, images, etc.)
- 🔍 **Détection intelligente** : Différenciation entre liens internes et externes
- 📊 **Statistiques détaillées** : Rapport complet sur les liens trouvés
- 💾 **Export JSON** : Sauvegarde structurée des résultats
- 🛡️ **Gestion SSL** : Support des sites HTTPS avec certificats invalides
- ⚡ **Performance optimisée** : Headers réalistes pour éviter les blocages
- 🎯 **Filtrage intelligent** : Exclusion automatique des liens non pertinents

## 📦 Installation

### Prérequis

- Go 1.16 ou supérieur
- Git

### Étapes d'installation

1. **Cloner le repository**
```bash
git clone https://github.com/alaminedione/web-link-scraper.git
cd web-link-scraper
```


2. **Compiler le projet**
```bash
go mod tidy
go build -o link-scraper main.go
```

## 🚀 Utilisation

### Syntaxe de base

```bash
./link-scraper <URL> [max_depth] [output_folder]
```

### Paramètres

| Paramètre | Description | Valeur par défaut |
|-----------|-------------|-------------------|
| `URL` | L'URL du site web à analyser | *Obligatoire* |
| `max_depth` | Profondeur maximale de récursion | `1` |
| `output_folder` | Dossier de sauvegarde des résultats | `./scraping_results` |

### Exemples d'utilisation

**Scraping simple (profondeur 1)**
```bash
./link-scraper https://example.com
```

**Scraping en profondeur**
```bash
./link-scraper https://example.com 3
```

**Scraping avec dossier de sortie personnalisé**
```bash
./link-scraper https://example.com 2 ./mes-resultats
```

## 📊 Classification des liens

Le scraper classe automatiquement les liens trouvés dans les catégories suivantes :

### 📄 Pages HTML
- Extensions : `.html`, `.htm`, `.php`, `.asp`, `.aspx`, `.jsp`
- Pages web classiques et dynamiques

### 📑 Documents
- Extensions : `.pdf`, `.doc`, `.docx`, `.xls`, `.xlsx`, `.ppt`, `.pptx`
- Fichiers bureautiques et documents

### 🖼️ Images
- Extensions : `.jpg`, `.jpeg`, `.png`, `.gif`, `.svg`, `.webp`
- Tous types d'images web

### ⚙️ Scripts
- Extensions : `.js`, `.mjs`, `.ts`
- Fichiers JavaScript et TypeScript

### 🎨 Feuilles de style
- Extensions : `.css`, `.scss`, `.sass`, `.less`
- Fichiers de style

### 🎬 Multimédia
- Extensions : `.mp4`, `.mp3`, `.avi`, `.mov`, `.wav`
- Fichiers audio et vidéo

### 📦 Archives
- Extensions : `.zip`, `.rar`, `.7z`, `.tar`, `.gz`
- Fichiers compressés

### ❓ Autres
- Tous les autres types de fichiers

## 📁 Structure des résultats

Les résultats sont sauvegardés dans un dossier horodaté :

```
scraping_results/
└── example_com_20240127_143022/
    ├── summary.json          # Résumé complet
    ├── html_pages.json       # Liste des pages HTML
    ├── documents.json        # Liste des documents
    ├── images.json          # Liste des images
    ├── scripts.json         # Liste des scripts
    ├── stylesheets.json     # Liste des CSS
    ├── multimedia.json      # Liste des médias
    └── archives.json        # Liste des archives
```

### Format du fichier summary.json

```json
{
  "base_url": "https://example.com",
  "total_links": 150,
  "internal_links": ["..."],
  "external_links": ["..."],
  "classified_links": {
    "html_pages": [...],
    "documents": [...],
    "images": [...]
  },
  "category_summary": {
    "html_pages": 45,
    "documents": 12,
    "images": 78
  },
  "statistics": {
    "pages_visited": 25,
    "execution_time": "1m23s",
    "max_depth_reached": 3
  },
  "timestamp": "2024-01-27 14:30:22"
}
```

## 🖥️ Exemples de sortie console

```
🚀 Starting ultra-fast scraping of: https://example.com
📊 Maximum depth: 2
💾 Output directory: ./scraping_results
--------------------------------------------------
🔗 Testing connection to https://example.com...
✅ Connection successful (Status: 200)
🔍 [Depth 0] Scraping: https://example.com
✅ Page loaded successfully: https://example.com
📊 Total of 45 links found on this page
🔍 [Depth 1] Scraping: https://example.com/about
✅ Page loaded successfully: https://example.com/about
📊 Total of 23 links found on this page

==================================================
📊 DETAILED STATISTICS
==================================================
🌐 Website: https://example.com
⏱️  Execution Time: 15.2s
📄 Pages Visited: 15
🔗 Total Links: 234
🏠 Internal Links: 189
🌍 External Links: 45
📊 Max Depth Reached: 2

📂 LINKS BY CATEGORY:
   📄 Html_pages: 67
   📑 Documents: 23
   🖼️ Images: 89
   ⚙️ Scripts: 12
   🎨 Stylesheets: 8
   🎬 Multimedia: 5

💾 Results saved to: ./scraping_results/example_com_20240127_143022
✅ Scraping completed successfully!
```

## ⚙️ Configuration avancée

### Modification des catégories

Pour ajouter ou modifier les catégories de fichiers, éditez la variable `fileExtensions` dans le code :

```go
var fileExtensions = map[LinkCategory][]string{
    CategoryHTML:       {".html", ".htm", ".php"},
    CategoryDocument:   {".pdf", ".doc", ".docx"},
    // Ajoutez vos extensions ici
}
```

### Paramètres de timeout

Pour modifier le timeout des requêtes HTTP :

```go
client := &http.Client{
    Transport: tr,
    Timeout:   30 * time.Second, // Modifier ici
}
```

## 🔧 Dépannage

**Erreur SSL/TLS**
- Le scraper ignore automatiquement les erreurs de certificat SSL
- Pour désactiver cette fonctionnalité, modifiez `InsecureSkipVerify: false`

**Timeout sur sites lents**
- Augmentez la valeur du timeout dans la configuration du client HTTP

**Blocage par le serveur**
- Le scraper utilise des headers réalistes pour éviter la détection
- Vous pouvez ajouter un délai entre les requêtes si nécessaire


## 🤝 Contribution

Les contributions sont les bienvenues ! Pour contribuer :

1. Fork le projet
2. Créez votre branche (`git checkout -b feature/AmazingFeature`)
3. Committez vos changements (`git commit -m 'Add some AmazingFeature'`)
4. Push vers la branche (`git push origin feature/AmazingFeature`)
5. Ouvrez une Pull Request

## 📝 Licence

Ce projet est sous licence MIT. Voir le fichier [LICENSE](LICENSE) pour plus de détails.


---

Développé avec ❤️ en Go
