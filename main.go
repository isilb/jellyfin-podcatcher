package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	// gofeed handles robust cross-parsing of RSS 1.0, 2.0, and Atom XML protocols.
	"github.com/mmcdole/gofeed"
)

// Config represents the internal runtime schema. It holds variables injected
// from the host environment alongside the feed layout parsed from config.json.
type Config struct {
	LibraryDir     string        `json:"library_dir"`
	JellyfinURL    string        `json:"jellyfin_url"`
	JellyfinAPIKey string        `json:"jellyfin_api_key"`
	RetentionDays  int           `json:"retention_days"`
	Feeds          []PodcastFeed `json:"feeds"`
}

// PodcastFeed binds individual show mapping configurations.
type PodcastFeed struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// JellyfinUser captures user profiles to run individual behavioral audits.
type JellyfinUser struct {
	ID   string `json:"Id"`
	Name string `json:"Name"`
}

// JellyfinItem parses explicit media fields saved inside the server database.
type JellyfinItem struct {
	Path     string `json:"Path"`
	UserData struct {
		IsFavorite bool `json:"IsFavorite"` // True if a user pinned this track with a star or heart icon
	} `json:"UserData"`
}

// JellyfinItemsResponse mirrors the payload envelope returned during recursive filtered queries.
type JellyfinItemsResponse struct {
	Items []JellyfinItem `json:"Items"`
}

// Global reference mapping the active text transaction journal location.
var logFilePath string

func main() {
	// Look up the running location of the active executable binary.
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Error locating executable path: %v\n", err)
		os.Exit(1)
	}
	
	// Anchor file interactions safely relative to the system binary location.
	baseDir := filepath.Dir(exePath)
	configPath := filepath.Join(baseDir, "config.json")
	logFilePath = filepath.Join(baseDir, "activity.log")

	logEvent("SYSTEM", "Daemonized podcatcher initialization complete. Entering execution loop.")

	// Continuous loop managing background runtime tasks.
	for {
		// Hot-reload config.json at the start of every cycle to process feed additions on the fly.
		cfg, err := loadConfig(configPath)
		if err != nil {
			logEvent("ERROR", fmt.Sprintf("Failed to read configuration matrix: %v", err))
		} else {
			now := time.Now().UTC()
			startDate := now.AddDate(0, 0, -cfg.RetentionDays) // Important for initial import
			endDate := now

			logEvent("SYSTEM", fmt.Sprintf("Starting update evaluation. Media Lookback Range: %s -> %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")))
			logEvent("SYSTEM", fmt.Sprintf("Current settings profile: Storage=%s | Retention=%d Days", cfg.LibraryDir, cfg.RetentionDays))

			// Connect to Jellyfin to track globally favorited media paths to exempt them from pruning.
			protectedPaths := fetchJellyfinFavorites(cfg)

			// Process individual show definitions sequentionally.
			for _, feed := range cfg.Feeds {
				if feed.URL == "" {
					logEvent("WARN", fmt.Sprintf("Skipping target entry [%s] - Empty destination URL metadata", feed.Name))
					continue
				}
				if err := processFeed(cfg, feed, startDate, endDate, protectedPaths); err != nil {
					logEvent("ERROR", fmt.Sprintf("Sync execution failed for target %s: %v", feed.Name, err))
				}
			}

			// Push a re-indexing notification to Jellyfin if an API token configuration is valid.
			if cfg.JellyfinAPIKey != "" {
				triggerJellyfinScan(cfg)
			}
		}

		// Block execution process. Holds memory footprint clean for exactly 1 hour.
		logEvent("SYSTEM", "Sync block finished processing successfully. Sleeping for 1 hour.")
		time.Sleep(1 * time.Hour)
	}
}

// logEvent drops text entries to standard output and writes them to a local transaction log file.
func logEvent(action, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, action, message)
	
	// Write immediately to stdout for real-time visibility under journalctl
	fmt.Print(logLine)

	// Append transactions directly to your local file mapping layer.
	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Failed to append metadata records to filesystem: %v\n", err)
		return
	}
	defer f.Close()
	_, _ = f.WriteString(logLine)
}

// loadConfig opens the local configuration file and overlays environment parameters derived
// from the host system service unit configuration.
func loadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg Config
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return nil, err
	}

	// Priority Check 1: Extract destination storage directory from system environment
	if envDir := os.Getenv("PODCAST_LIBRARY_DIR"); envDir != "" {
		cfg.LibraryDir = envDir
	} else if cfg.LibraryDir == "" {
		cfg.LibraryDir = "/media/media/podcasts" // Fail-safe standard layout fallback
	}

	// Priority Check 2: Extract retention windows from system environment
	if envRetention := os.Getenv("PODCAST_RETENTION_DAYS"); envRetention != "" {
		if days, err := strconv.Atoi(envRetention); err == nil {
			cfg.RetentionDays = days
		}
	} else if cfg.RetentionDays == 0 {
		cfg.RetentionDays = 14 // Fall-safe tracking default (2 weeks)
	}

	// Priority Check 3: Extract Jellyfin network connection definitions
	if envURL := os.Getenv("JELLYFIN_URL"); envURL != "" {
		cfg.JellyfinURL = envURL
	}
	if envKey := os.Getenv("JELLYFIN_API_KEY"); envKey != "" {
		cfg.JellyfinAPIKey = envKey
	}

	return &cfg, nil
}

// fetchJellyfinFavorites aggregates interactive user actions to find files to protect from pruning.
func fetchJellyfinFavorites(cfg *Config) map[string]bool {
	favoritesMap := make(map[string]bool)
	if cfg.JellyfinAPIKey == "" || cfg.JellyfinURL == "" {
		return favoritesMap
	}

	client := &http.Client{Timeout: 12 * time.Second}

	// 1. Fetch user records online.
	usersURL := fmt.Sprintf("%s/Users", cfg.JellyfinURL)
	req, err := http.NewRequest("GET", usersURL, nil)
	if err != nil {
		return favoritesMap
	}
	req.Header.Set("X-MediaBrowser-Token", cfg.JellyfinAPIKey)

	resp, err := client.Do(req)
	if err != nil {
		logEvent("WARN", fmt.Sprintf("Failed to query users from Jellyfin: %v", err))
		return favoritesMap
	}
	defer resp.Body.Close()

	var users []JellyfinUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return favoritesMap
	}

	// 2. Discover starred tracks across discovered accounts.
	for _, user := range users {
		itemsURL := fmt.Sprintf("%s/Users/%s/Items?Recursive=true&Filters=IsFavorite&IncludeItemTypes=AudioBook,Audio", cfg.JellyfinURL, user.ID)
		itemReq, err := http.NewRequest("GET", itemsURL, nil)
		if err != nil {
			continue
		}
		itemReq.Header.Set("X-MediaBrowser-Token", cfg.JellyfinAPIKey)

		itemResp, err := client.Do(itemReq)
		if err != nil {
			continue
		}

		var itemsData JellyfinItemsResponse
		if err := json.NewDecoder(itemResp.Body).Decode(&itemsData); err == nil {
			for _, item := range itemsData.Items {
				if item.Path != "" {
					cleanedPath := filepath.Clean(item.Path)
					favoritesMap[cleanedPath] = true
				}
			}
		}
		itemResp.Body.Close()
	}

	if len(favoritesMap) > 0 {
		logEvent("JELLYFIN", fmt.Sprintf("Cached %d globally starred/flagged items for retention preservation.", len(favoritesMap)))
	}
	return favoritesMap
}

// processFeed evaluates a show directory target, fires down retention clean sweeps, and checks for updates.
func processFeed(cfg *Config, feed PodcastFeed, startDate, endDate time.Time, protectedPaths map[string]bool) error {
	showDir := filepath.Join(cfg.LibraryDir, feed.Name)
	if err := os.MkdirAll(showDir, 0755); err != nil {
		return err
	}

	// Drop expired assets before processing network streams.
	if err := cleanOldEpisodes(showDir, cfg.RetentionDays, protectedPaths); err != nil {
		logEvent("WARN", fmt.Sprintf("Pruning engine warning for show %s: %v", feed.Name, err))
	}

	fp := gofeed.NewParser()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	parsedFeed, err := fp.ParseURLWithContext(feed.URL, ctx)
	if err != nil {
		return err
	}

	for _, item := range parsedFeed.Items {
		if item.PublishedParsed == nil {
			continue
		}
		pubDate := item.PublishedParsed.UTC()

		// Verify publication dates fall within the active lookup execution window.
		if pubDate.After(startDate) && pubDate.Before(endDate) {
			audioURL := ""
			for _, enc := range item.Enclosures {
				if strings.Contains(enc.Type, "audio") || strings.Contains(enc.URL, ".mp3") || strings.Contains(enc.URL, ".m4a") {
					audioURL = enc.URL
					break
				}
			}
			if audioURL == "" {
				continue
			}

			safeTitle := sanitizeFilename(item.Title)
			filename := fmt.Sprintf("%s - %s.mp3", pubDate.Format("2006-01-02"), safeTitle)
			filePath := filepath.Join(showDir, filename)

			// Duplicate tracking check block. Skip streaming if file already exists locally.
			if _, err := os.Stat(filePath); err == nil {
				continue 
			}

			if err := downloadFile(audioURL, filePath); err != nil {
				logEvent("ERROR", fmt.Sprintf("Stream acquisition failed for file [%s]: %v", filename, err))
				continue
			}

			logEvent("DOWNLOAD", fmt.Sprintf("Pulled file asset down: %s/%s", feed.Name, filename))

			// Write matching local metadata files to avoid needing external scraper queries later.
			if err := writeNFO(filePath, item.Title, item.Description, pubDate); err != nil {
				logEvent("WARN", fmt.Sprintf("Failed to write metadata sidecar XML descriptor for track: %s", filename))
			}
		}
	}
	return nil
}

// cleanOldEpisodes clears tracks out when they exceed chronological age requirements,
// unless they match active preservation hashes or local tracking tokens.
func cleanOldEpisodes(dir string, retentionDays int, protectedPaths map[string]bool) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	// Check for a folder-level ".keep" override file. If found, skip pruning for this folder entirely.
	if _, err := os.Stat(filepath.Join(dir, ".keep")); err == nil {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// Bypass inner system directories or manually injected file exception flags.
		if entry.IsDir() || entry.Name() == ".keep" || strings.Contains(entry.Name(), "[KEEP]") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Prune the item if its timestamp is older than our configuration cutoff date.
		if info.ModTime().Before(cutoff) {
			mediaFilePath := filepath.Clean(filepath.Join(dir, entry.Name()))

			// Cross-reference against Jellyfin favorites. If the user favorited this file, save it.
			if protectedPaths[mediaFilePath] {
				continue 
			}

			err := os.Remove(mediaFilePath)
			if err == nil {
				logEvent("DELETE", fmt.Sprintf("Evicted stale historical item from retention cache: %s/%s", filepath.Base(dir), entry.Name()))
				
				// Purge accompanying companion sidecar NFO files cleanly.
				nfoPath := strings.TrimSuffix(mediaFilePath, filepath.Ext(mediaFilePath)) + ".nfo"
				_ = os.Remove(nfoPath)
			}
		}
	}
	return nil
}

// downloadFile builds an HTTP connection channel with a generous 20-minute connection window for downloading large media files.
func downloadFile(url, destPath string) error {
	client := &http.Client{Timeout: 20 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("remote asset server returned error code: %s", resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// writeNFO compiles and drops localized Kodi-compliant NFO sidecars.
func writeNFO(mediaPath, title, summary string, pubDate time.Time) error {
	nfoPath := strings.TrimSuffix(mediaPath, filepath.Ext(mediaPath)) + ".nfo"
	xmlContent := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<entry>
  <title>%s</title>
  <summary>%s</summary>
  <pubdate>%s</pubdate>
</entry>`, escapeXML(title), escapeXML(summary), pubDate.Format("2006-01-02"))
	return os.WriteFile(nfoPath, []byte(xmlContent), 0644)
}

// triggerJellyfinScan pings Jellyfin to trigger a library scan.
func triggerJellyfinScan(cfg *Config) {
	url := fmt.Sprintf("%s/Library/Refresh", cfg.JellyfinURL)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("X-MediaBrowser-Token", cfg.JellyfinAPIKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logEvent("WARN", fmt.Sprintf("Jellyfin control interface communication error: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		logEvent("JELLYFIN", "Media indexing notification delivered successfully.")
	}
}

// sanitizeFilename filters characters that are illegal or problematic across standard Linux layout clusters.
func sanitizeFilename(name string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9\s\.\_\-]`)
	return strings.TrimSpace(reg.ReplaceAllString(name, ""))
}

// escapeXML maps dangerous punctuation to clean, validated escape structures.
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
