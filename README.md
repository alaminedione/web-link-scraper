# Get Links - Un Scraper de Liens Web en Go

`Get Links` est un outil de scraping de liens web développé en Go. Il permet d'explorer un site web de manière récursive, d'extraire tous les liens (internes et externes), et de sauvegarder les résultats dans un fichier JSON. L'outil est conçu pour être robuste, gérant les redirections, les erreurs HTTP et les différents encodages de contenu.

## Fonctionnalités

- **Scraping Récursif**: Explore les liens internes jusqu'à une profondeur spécifiée.
- **Extraction Complète des Liens**: Identifie et catégorise les liens internes et externes.
- **Gestion des Erreurs**: Gère les erreurs HTTP et les problèmes de parsing HTML.
- **Headers Réalistes**: Utilise des en-têtes HTTP réalistes pour minimiser les blocages.
- **Normalisation des URL**: Nettoie et normalise les URL pour éviter les doublons et les paramètres de suivi.
- **Rapports Détaillés**: Fournit des statistiques complètes sur le scraping, y compris le nombre de pages visitées, les liens trouvés et le temps d'exécution.
- **Sauvegarde des Résultats**: Exporte les résultats au format JSON pour une analyse ultérieure.

## Utilisation

### Prérequis

Assurez-vous d'avoir Go installé sur votre machine.

### Exécution

#### Build

Pour compiler l'exécutable, utilisez la commande suivante :

```bash
go build -o get-links main.go
```

Cela créera un exécutable nommé `get-links` (ou `get-links.exe` sur Windows) dans le répertoire courant.

#### Lancer l'exécutable

Une fois compilé, vous pouvez exécuter l'outil directement :

```bash
./get-links <URL> [profondeur_max] [dossier_sortie]
```

#### Exécution directe (sans compilation)

Pour exécuter le scraper sans le compiler au préalable, utilisez la commande suivante :

```bash
go run main.go <URL> [profondeur_max] [dossier_sortie]
```

**Paramètres :**

- `<URL>`: L'URL du site web à scraper (obligatoire).
- `[profondeur_max]`: La profondeur maximale du scraping récursif (optionnel, par défaut : `1`).
- `[dossier_sortie]`: Le dossier où sauvegarder les résultats JSON (optionnel, par défaut : `./scraping_results`).

### Exemple

Scraper `https://example.com` jusqu'à une profondeur de `2` et sauvegarder les résultats dans le dossier `./my_results` :

```bash
go run main.go https://example.com 2 ./my_results
```

## Structure du Projet

- `main.go`: Contient la logique principale du scraper, y compris la structure des données, les fonctions de scraping, de normalisation des URL et de gestion des résultats.
- `go.mod` et `go.sum`: Fichiers de gestion des dépendances Go.

## Dépendances

- `github.com/PuerkitoBio/goquery`: Pour le parsing HTML et la sélection d'éléments.