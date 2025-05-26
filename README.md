# Get Links - An Ultra-Fast Web Link Scraper in Go

`Get Links` is a web link scraping tool developed in Go. It allows you to recursively explore a website, extract all links (internal and external), and save the results to a JSON file. The tool is designed to be robust, handling redirects, HTTP errors, and various content encodings with ultra-fast performance.

## Features

- **Recursive Scraping**: Explores internal links up to a specified depth.
- **Comprehensive Link Extraction**: Identifies and categorizes both internal and external links.
- **Error Handling**: Manages HTTP errors and HTML parsing issues.
- **Realistic Headers**: Uses realistic HTTP headers to minimize blocking.
- **URL Normalization**: Cleans and normalizes URLs to avoid duplicates and tracking parameters.
- **Detailed Reports**: Provides comprehensive scraping statistics, including the number of pages visited, links found, and execution time.
- **Result Saving**: Exports results in JSON format for later analysis.

## Usage

### Prerequisites

Ensure you have Go installed on your machine.

### Execution

#### Build

To compile the executable, use the following command:

```bash
go build -o get-links main.go
```

This will create an executable named `get-links` (or `get-links.exe` on Windows) in the current directory.

#### Run the executable

Once compiled, you can run the tool directly:

```bash
./get-links <URL> [max_depth] [output_folder]
```

#### Direct execution (without compilation)

To run the scraper without compiling it first, use the following command:

```bash
go run main.go <URL> [max_depth] [output_folder]
```

**Parameters:**

- `<URL>`: The URL of the website to scrape (required).
- `[max_depth]`: The maximum depth for recursive scraping (optional, default: `1`).
- `[output_folder]`: The folder to save the JSON results (optional, default: `./scraping_results`).

**Understanding Depth (`max_depth`):**

In the context of `Get Links`, "depth" refers to the level of recursion the scraper will reach when exploring a website.

- **Depth 0**: The scraper only visits the initial URL provided. It does not follow any links found on that page.
- **Depth 1**: The scraper visits the initial URL (depth 0), and then follows all *internal* links found on that page, visiting them (depth 1). It will not follow links found on depth 1 pages.
- **Depth N**: The scraper continues to follow internal links up to `N` clicks away from the initial URL.

This parameter helps control the scope of the scraping, preventing it from exploring the entire internet and focusing on a relevant portion of the website.

### Example

Scrape `https://example.com` up to a depth of `2` and save the results to the `./my_results` folder:

```bash
go run main.go https://example.com 2 ./my_results
```

## Project Structure

- `main.go`: Contains the main scraper logic, including data structures, scraping functions, URL normalization, and result handling.
- `go.mod` and `go.sum`: Go dependency management files.

## Dependencies

- `github.com/PuerkitoBio/goquery`: For HTML parsing and element selection.