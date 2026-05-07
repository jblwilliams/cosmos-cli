package cosmos

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	graphqlEndpoint = "https://api.cosmos.so/graphql"
	pageSize        = 50
)

const graphqlQuery = `
query GetClusterElements($clusterId: ClusterId, $pageCursor: String, $pageSize: Int) {
  clusterConnections(
    clusterId: $clusterId
    meta: {pageSize: $pageSize, pageCursor: $pageCursor}
  ) {
    items {
      element {
        id
        __typename
        contentAccessibility
        shareUrl
        source {
          url
          __typename
        }
        ... on MediaElementTile {
          hasMoreMedia
          multipleMedia {
            mediaId
            url
            width
            height
            __typename
            ... on AnimatedImage {
              video { url thumbnailUrl __typename }
              __typename
            }
            ... on Video {
              mux {
                downloadableUrl: mp4Url(quality: HIGH)
                __typename
              }
              __typename
            }
          }
          media {
            mediaId
            url
            width
            height
            __typename
            ... on AnimatedImage {
              video { url thumbnailUrl __typename }
              __typename
            }
            ... on Video {
              mux {
                downloadableUrl: mp4Url(quality: HIGH)
                __typename
              }
              __typename
            }
          }
        }
      }
    }
    meta {
      nextPageCursor
      count
      __typename
    }
    __typename
  }
}
`

type DownloadOptions struct {
	OutputDir    string
	SkipExisting bool
	Delay        float64
}

type mediaURL struct {
	URL       string
	MediaType string
	MediaID   string
}

func getClusterID(client *http.Client, collectionURL string) (int, error) {
	fmt.Printf("Fetching collection page: %s\n", collectionURL)
	resp, err := client.Get(collectionURL)
	if err != nil {
		return 0, fmt.Errorf("fetching collection page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(`"clusterId"\s*:\s*(\d+)`)
	matches := re.FindSubmatch(body)
	if matches == nil {
		re = regexp.MustCompile(`clusterId["\s:]+(\d+)`)
		matches = re.FindSubmatch(body)
	}
	if matches == nil {
		return 0, fmt.Errorf("could not find clusterId in page — make sure the URL is a valid cosmos.so collection")
	}

	var id int
	fmt.Sscanf(string(matches[1]), "%d", &id)
	fmt.Printf("Found clusterId: %d\n", id)
	return id, nil
}

type graphqlResponse struct {
	Data struct {
		ClusterConnections struct {
			Items []json.RawMessage `json:"items"`
			Meta  struct {
				NextPageCursor *string `json:"nextPageCursor"`
				Count          int     `json:"count"`
			} `json:"meta"`
		} `json:"clusterConnections"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func fetchAllElements(client *http.Client, clusterID int) ([]json.RawMessage, error) {
	var allItems []json.RawMessage
	var cursor *string
	page := 1
	total := -1

	for {
		fmt.Printf("Fetching page %d... ", page)

		variables := map[string]any{
			"clusterId": clusterID,
			"pageSize":  pageSize,
		}
		if cursor != nil {
			variables["pageCursor"] = *cursor
		}

		payload, _ := json.Marshal(map[string]any{
			"operationName": "GetClusterElements",
			"variables":     variables,
			"query":         graphqlQuery,
		})

		resp, err := client.Post(
			graphqlEndpoint+"?q=GetClusterElements",
			"application/json",
			strings.NewReader(string(payload)),
		)
		if err != nil {
			return nil, fmt.Errorf("graphql request: %w", err)
		}

		var gqlResp graphqlResponse
		if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decoding graphql response: %w", err)
		}
		resp.Body.Close()

		if len(gqlResp.Errors) > 0 {
			return nil, fmt.Errorf("graphql error: %s", gqlResp.Errors[0].Message)
		}

		items := gqlResp.Data.ClusterConnections.Items
		meta := gqlResp.Data.ClusterConnections.Meta

		if total < 0 {
			total = meta.Count
			fmt.Printf("(total: %d elements)\n", total)
		} else {
			fmt.Printf("(%d/%d)\n", len(allItems)+len(items), total)
		}

		allItems = append(allItems, items...)
		cursor = meta.NextPageCursor
		if cursor == nil {
			break
		}

		page++
		time.Sleep(300 * time.Millisecond)
	}

	fmt.Printf("\nFetched %d elements total.\n", len(allItems))
	return allItems, nil
}

type element struct {
	Element struct {
		ID           string `json:"id"`
		Typename     string `json:"__typename"`
		HasMoreMedia bool   `json:"hasMoreMedia"`
		Media        *media `json:"media"`
		MultipleMedia []media `json:"multipleMedia"`
	} `json:"element"`
}

type media struct {
	MediaID  string `json:"mediaId"`
	URL      string `json:"url"`
	Typename string `json:"__typename"`
	Video    *struct {
		URL string `json:"url"`
	} `json:"video"`
	Mux *struct {
		DownloadableURL string `json:"downloadableUrl"`
	} `json:"mux"`
}

func extractMediaURLs(raw json.RawMessage) []mediaURL {
	var elem element
	if err := json.Unmarshal(raw, &elem); err != nil {
		return nil
	}

	e := elem.Element
	var urls []mediaURL

	switch e.Typename {
	case "MediaElementTile":
		mediaList := e.MultipleMedia
		if len(mediaList) == 0 && e.Media != nil {
			mediaList = []media{*e.Media}
		}

		for i, m := range mediaList {
			id := m.MediaID
			if id == "" {
				id = fmt.Sprintf("%s_%d", e.ID, i)
			}

			switch m.Typename {
			case "StaticImage":
				urls = append(urls, mediaURL{cleanURL(m.URL), "image", id})
			case "AnimatedImage":
				if m.Video != nil && m.Video.URL != "" {
					urls = append(urls, mediaURL{m.Video.URL, "video", id})
				} else {
					urls = append(urls, mediaURL{cleanURL(m.URL), "image", id})
				}
			case "Video":
				if m.Mux != nil && m.Mux.DownloadableURL != "" {
					urls = append(urls, mediaURL{m.Mux.DownloadableURL, "video", id})
				} else if m.URL != "" {
					urls = append(urls, mediaURL{m.URL, "video", id})
				}
			default:
				if m.URL != "" {
					urls = append(urls, mediaURL{cleanURL(m.URL), "image", id})
				}
			}
		}

	case "ProductElementTile", "WebsiteElementTile":
		if e.Media != nil && e.Media.URL != "" {
			id := e.Media.MediaID
			if id == "" {
				id = e.ID
			}
			urls = append(urls, mediaURL{cleanURL(e.Media.URL), "image", id})
		}
	}

	return urls
}

func cleanURL(u string) string {
	if i := strings.Index(u, "?"); i >= 0 {
		return u[:i]
	}
	return u
}

func fileExtension(dlURL, mediaType, contentType string) string {
	if mediaType == "video" {
		return ".mp4"
	}

	ctMap := map[string]string{
		"image/jpeg":    ".jpg",
		"image/jpg":     ".jpg",
		"image/png":     ".png",
		"image/gif":     ".gif",
		"image/webp":    ".webp",
		"image/avif":    ".avif",
		"image/svg+xml": ".svg",
	}
	for ct, ext := range ctMap {
		if strings.Contains(contentType, ct) {
			return ext
		}
	}

	parsed, err := url.Parse(dlURL)
	if err == nil {
		ext := strings.ToLower(filepath.Ext(parsed.Path))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif", ".svg", ".mp4", ".mov":
			return ext
		}
	}

	return ".jpg"
}

var unsafeChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

func sanitize(name string) string {
	return unsafeChars.ReplaceAllString(name, "_")
}

func downloadFile(client *http.Client, dlURL, dest string, retries int) bool {
	for attempt := range retries {
		resp, err := client.Get(dlURL)
		if err != nil {
			if attempt < retries-1 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			fmt.Printf("  [ERROR] Failed to download %s: %v\n", dlURL, err)
			return false
		}

		if resp.StatusCode == 404 {
			resp.Body.Close()
			fmt.Printf("  [404] Not found: %s\n", dlURL)
			return false
		}

		if resp.StatusCode >= 400 {
			resp.Body.Close()
			if attempt < retries-1 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			fmt.Printf("  [ERROR] HTTP %d for %s\n", resp.StatusCode, dlURL)
			return false
		}

		finalDest := dest
		if strings.HasSuffix(dest, ".jpg") {
			ct := resp.Header.Get("Content-Type")
			ext := fileExtension(dlURL, "image", ct)
			if ext != ".jpg" {
				finalDest = strings.TrimSuffix(dest, ".jpg") + ext
			}
		}

		if err := os.MkdirAll(filepath.Dir(finalDest), 0o755); err != nil {
			resp.Body.Close()
			fmt.Printf("  [ERROR] mkdir: %v\n", err)
			return false
		}

		f, err := os.Create(finalDest)
		if err != nil {
			resp.Body.Close()
			fmt.Printf("  [ERROR] create file: %v\n", err)
			return false
		}

		_, err = io.Copy(f, resp.Body)
		resp.Body.Close()
		f.Close()
		if err != nil {
			fmt.Printf("  [ERROR] writing file: %v\n", err)
			return false
		}
		return true
	}
	return false
}

func DownloadCollection(client *http.Client, collectionURL string, opts DownloadOptions) error {
	parsed, err := url.Parse(collectionURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid collection URL: expected https://www.cosmos.so/username/collection-name")
	}

	username := parts[0]
	collectionSlug := parts[1]
	collectionName := username + "_" + collectionSlug

	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join("cosmos_downloads", sanitize(collectionName))
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Println("cosmos.so Collection Downloader")
	fmt.Printf("%s\n", strings.Repeat("=", 60))
	fmt.Printf("Collection: %s/%s\n", username, collectionSlug)
	abs, _ := filepath.Abs(outputDir)
	fmt.Printf("Output directory: %s\n", abs)
	fmt.Printf("%s\n\n", strings.Repeat("=", 60))

	clusterID, err := getClusterID(client, collectionURL)
	if err != nil {
		return err
	}

	fmt.Println("\nFetching elements from collection...")
	elements, err := fetchAllElements(client, clusterID)
	if err != nil {
		return err
	}

	fmt.Println("\nDownloading images...")
	fmt.Println(strings.Repeat("─", 60))

	stats := struct{ downloaded, skipped, failed, slideshows int }{}

	for idx, raw := range elements {
		num := idx + 1

		var elem element
		json.Unmarshal(raw, &elem)
		if elem.Element.HasMoreMedia {
			stats.slideshows++
		}

		urls := extractMediaURLs(raw)
		if len(urls) == 0 {
			continue
		}

		delay := time.Duration(opts.Delay * float64(time.Second))

		if len(urls) > 1 {
			elemDir := filepath.Join(outputDir, fmt.Sprintf("%04d_%s_slideshow", num, elem.Element.ID))
			for imgIdx, mu := range urls {
				filename := fmt.Sprintf("%02d_%s.jpg", imgIdx+1, sanitize(mu.MediaID))
				dest := filepath.Join(elemDir, filename)

				if opts.SkipExisting {
					if _, err := os.Stat(dest); err == nil {
						stats.skipped++
						continue
					}
				}

				truncURL := mu.URL
				if len(truncURL) > 60 {
					truncURL = truncURL[:60] + "..."
				}
				fmt.Printf("[%d/%d] Slideshow %d/%d: %s\n", num, len(elements), imgIdx+1, len(urls), truncURL)

				if downloadFile(client, mu.URL, dest, 3) {
					stats.downloaded++
				} else {
					stats.failed++
				}
				time.Sleep(delay)
			}
		} else {
			mu := urls[0]
			ext := ".jpg"
			if mu.MediaType == "video" {
				ext = ".mp4"
			}
			filename := fmt.Sprintf("%04d_%s%s", num, sanitize(mu.MediaID), ext)
			dest := filepath.Join(outputDir, filename)

			if opts.SkipExisting {
				if _, err := os.Stat(dest); err == nil {
					stats.skipped++
					continue
				}
			}

			truncURL := mu.URL
			if len(truncURL) > 70 {
				truncURL = truncURL[:70] + "..."
			}
			fmt.Printf("[%d/%d] %s\n", num, len(elements), truncURL)

			if downloadFile(client, mu.URL, dest, 3) {
				stats.downloaded++
			} else {
				stats.failed++
			}
			time.Sleep(delay)
		}
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Println("Download Complete!")
	fmt.Printf("%s\n", strings.Repeat("=", 60))
	fmt.Printf("  Downloaded:      %d files\n", stats.downloaded)
	fmt.Printf("  Skipped:         %d files (already existed)\n", stats.skipped)
	fmt.Printf("  Failed:          %d files\n", stats.failed)
	fmt.Printf("  Slideshow items: %d (each may have multiple images)\n", stats.slideshows)
	fmt.Printf("  Output:          %s\n", abs)
	fmt.Printf("%s\n\n", strings.Repeat("=", 60))

	return nil
}
