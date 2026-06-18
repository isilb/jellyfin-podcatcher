package main

import (
        "context"
        "encoding/json"
        "fmt"
        "html/template"
        "io"
		//"log"
        "net/http"
        "net/url"
		"image"
    	"image/color"
    	"image/draw"
    	"image/jpeg"
        "os"
        "path/filepath"
        "regexp"
        "strconv"
        "strings"
		"sync"
        "time"

        // gofeed handles robust cross-parsing of RSS 1.0, 2.0, and Atom XML protocols.
        "github.com/mmcdole/gofeed"
)

var (
	configMutex sync.Mutex
	logMutex    sync.Mutex
	logFile  *os.File
)

var syncTrigger = make(chan bool, 1)

// Config represents the internal runtime schema. It holds variables injected
// from the host environment alongside the feed layout parsed from config.json.
type Config struct {
        LibraryDir        string        `json:"library_dir"`
        JellyfinURL       string        `json:"jellyfin_url"`
        JellyfinAPIKey    string        `json:"jellyfin_api_key"`
        ReindexWebhookURL string        `json:"reindex_webhook_url"` // Platform-agnostic webhook destination
        RetentionDays     int           `json:"retention_days"`
        Feeds             []PodcastFeed `json:"feeds"`
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

// Create data structure for agnostic media servers (Plex, Emby, etc.)
type MediaServer interface {
    GetProtectedPaths() (map[string]bool, error) // Match the return types
    TriggerRefresh() error                       // Rename to match
}

// Global reference mapping the active text transaction journal location.
var logFilePath string

<<<<<<< HEAD
type JellyfinProvider struct {
    Cfg *Config
}

func (j *JellyfinProvider) GetProtectedPaths() (map[string]bool, error) {
    // Moved the logic from fetchJellyfinFavorites here
    return fetchJellyfinFavorites(j.Cfg), nil
}

func (j *JellyfinProvider) TriggerRefresh() error {
    triggerJellyfinScan(j.Cfg)
    return nil
}

// Thread-Safe and Persistent Mutex Handling
func initLogger(path string) {
    var err error
    logFile, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Fatal: Could not open log file: %v\n", err)
        os.Exit(1)
    }
=======
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
>>>>>>> 21dce12d5c634dcf426f1208ea4752bbd64d68f7
}

func logEvent(action, message string) {
    logMutex.Lock()
    defer logMutex.Unlock()
    
    timestamp := time.Now().Format("2006-01-02 15:04:05")
    logLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, action, message)
    
    fmt.Print(logLine) // Standard out for journalctl
    if logFile != nil {
        logFile.WriteString(logLine)
    }
}

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

    initLogger(logFilePath) 
    logEvent("SYSTEM", "Daemonized podcatcher initialization complete. Entering execution loop.")

    // Spin up the Web UI and Search Handler in a background thread
    go startWebServer(baseDir, configPath)

    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()

    // Continuous loop managing background runtime tasks.
    for {
        // 1. Hot-reload config
        configMutex.Lock()
        cfg, err := loadConfig(configPath)
        configMutex.Unlock()

        if err != nil {
            logEvent("ERROR", fmt.Sprintf("Failed to read config.json matrix: %v", err))
        } else {
            // 2. Perform the sync via the interface
            // This replaces all your manual feed processing logic
            provider := &JellyfinProvider{Cfg: cfg}
            
            logEvent("SYSTEM", fmt.Sprintf("Starting update evaluation. Retention: %d Days", cfg.RetentionDays))
            
            runSync(provider, cfg, time.Now().AddDate(0, 0, -cfg.RetentionDays), time.Now())
        }

        // 3. Wait for either a manual trigger or the ticker
        select {
        case <-syncTrigger:
            logEvent("SYSTEM", "Manual sync triggered by user.")
        case <-ticker.C:
            logEvent("SYSTEM", "Scheduled sync time reached.")
        }
    }
}

// Get which media server is being used (Jellyfin, Emby,Plex, etc)
func runSync(provider MediaServer, cfg *Config, startDate, endDate time.Time) {
    // 1. Fetch paths via the interface (Jellyfin/Plex/etc)
    protectedPaths, err := provider.GetProtectedPaths()
    if err != nil {
        logEvent("ERROR", fmt.Sprintf("Provider failed to fetch protected paths: %v", err))
    }

    // 2. Process feeds
    for _, feed := range cfg.Feeds {
        if err := processFeed(cfg, feed, startDate, endDate, protectedPaths); err != nil {
            logEvent("ERROR", fmt.Sprintf("Sync failed for %s: %v", feed.Name, err))
        }
    }

    // 3. Trigger refresh via the interface
    if err := provider.TriggerRefresh(); err != nil {
        logEvent("ERROR", fmt.Sprintf("Refresh trigger failed: %v", err))
    }
}

// startWebServer manages the live API proxy endpoints and handles reading/writing changes to config.json.
func startWebServer(baseDir, configPath string) {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    // 1. Lock and Load: Get current config data
    configMutex.Lock()
    cfg, _ := loadConfig(configPath)
    configMutex.Unlock()

    if cfg == nil {
        cfg = &Config{Feeds: []PodcastFeed{}, RetentionDays: 14}
    }

    // 2. Fetch Helper Data: Load logs
    logsData, _ := os.ReadFile(logFilePath) 
    logsContent := string(logsData)

    // 3. Prepare the Data Struct: Do this BEFORE executing the template
    // 3. Prepare the Data Struct
	data := struct {
		Feeds         []PodcastFeed
		Logs          string
		RetentionDays int
		JellyfinURL   string
		APIKey        string // Added this
	}{
		Feeds:         cfg.Feeds,
		Logs:          logsContent,
		RetentionDays: cfg.RetentionDays,
		JellyfinURL:   cfg.JellyfinURL,
		APIKey:        cfg.JellyfinAPIKey, // Mapping the field
	}

    // 4. Parse and Execute: Now the data is ready to be injected
    tmplPath := filepath.Join(baseDir, "index.html")
    tmpl, err := template.ParseFiles(tmplPath)
    if err != nil {
        logEvent("ERROR", fmt.Sprintf("Template parse error: %v", err))
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }
    
    tmpl.Execute(w, data)
	})

    // 2. Search API
    http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
        query := r.URL.Query().Get("q")
    
    	// Always set headers early
    	w.Header().Set("Content-Type", "application/json")

    	if query == "" {
        	w.Write([]byte(`{"results":[]}`))
        	return
    	}

    	apiURL := "https://itunes.apple.com/search?media=podcast&term=" + url.QueryEscape(query)
    	resp, err := http.Get(apiURL)
    	if err != nil {
        	logEvent("ERROR", fmt.Sprintf("External search failed: %v", err))
        	w.Write([]byte(`{"results":[]}`)) // Return empty JSON instead of crashing
        	return
    	}
    	defer resp.Body.Close()

    	// Stream the body directly
    	io.Copy(w, resp.Body)
    	})

    // 3. Add API Endpoint (Corrected)
    http.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
        name := r.URL.Query().Get("name")
        url := r.URL.Query().Get("url")

        if name == "" || url == "" {
            http.Error(w, "Missing parameters", http.StatusBadRequest)
            return
        }

        configMutex.Lock()
        defer configMutex.Unlock()

        // 1. Reload latest config
        cfg,err := loadConfig(configPath)
        if cfg == nil {
            cfg = &Config{Feeds: []PodcastFeed{}}
        }

        // 2. Append new feed
        cfg.Feeds = append(cfg.Feeds, PodcastFeed{Name: name, URL: url})

        // 3. Save to disk
        file, _ := json.MarshalIndent(cfg, "", "  ")
        err = os.WriteFile(configPath, file, 0644)
        if err != nil {
            logEvent("ERROR", fmt.Sprintf("Failed to save: %v", err))
            http.Error(w, "Save failed", http.StatusInternalServerError)
            return
        }

        logEvent("SYSTEM", fmt.Sprintf("Added new feed: %s", name))
        w.WriteHeader(http.StatusOK)
    })

	// 4. Save Feeds Handler
    http.HandleFunc("/save", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        // Parse form data
        r.ParseForm()
        names := r.Form["name"]
        urls := r.Form["url"]

        configMutex.Lock()
        defer configMutex.Unlock()

        cfg, err := loadConfig(configPath)
        if err != nil {
            http.Error(w, "Config error", http.StatusInternalServerError)
            return
        }

        // Rebuild feeds list from the form input
        var newFeeds []PodcastFeed
        for i := 0; i < len(names); i++ {
            if names[i] != "" && urls[i] != "" {
                newFeeds = append(newFeeds, PodcastFeed{Name: names[i], URL: urls[i]})
            }
        }
        cfg.Feeds = newFeeds

        // Save to disk
        file, _ := json.MarshalIndent(cfg, "", "  ")
        err = os.WriteFile(configPath, file, 0644)
        if err != nil {
            http.Error(w, "Save failed", http.StatusInternalServerError)
            return
        }


        // Redirect back to main page
        http.Redirect(w, r, "/", http.StatusSeeOther)
    })

	// UPDATE SETTINGS HANDLER
http.HandleFunc("/update-settings", func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // 1. Parse form data
    r.ParseForm()
    retentionStr := r.FormValue("retention")
    jellyfinURL := r.FormValue("jellyfin_url")
    apiKey := r.FormValue("api_key")

    // 2. Validate and Parse
    days, err := strconv.Atoi(retentionStr)
    if err != nil {
        http.Error(w, "Invalid retention days", http.StatusBadRequest)
        return
    }

    // 3. Lock and Update
    configMutex.Lock()
    defer configMutex.Unlock()

    cfg, err := loadConfig(configPath)
    if err != nil {
        http.Error(w, "Failed to load config", http.StatusInternalServerError)
        return
    }

    // Update fields
    cfg.RetentionDays = days
    cfg.JellyfinURL = jellyfinURL
    cfg.JellyfinAPIKey = apiKey

    // 4. Save to disk
    file, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        http.Error(w, "Failed to encode config", http.StatusInternalServerError)
        return
    }

    err = os.WriteFile(configPath, file, 0644)
    if err != nil {
        http.Error(w, "Failed to save config file", http.StatusInternalServerError)
        return
    }

    logEvent("SYSTEM", fmt.Sprintf("Settings updated: Retention=%d, URL=%s", days, jellyfinURL))
    w.WriteHeader(http.StatusOK)
})

//Manual Sync Button
http.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
    select {
    case syncTrigger <- true:
        w.WriteHeader(http.StatusOK)
    default:
        // Channel already full, sync is already in progress
        w.WriteHeader(http.StatusAccepted)
    }
})

    logEvent("SYSTEM", "Admin Web UI API engine successfully bound to http://0.0.0.0:8080")
    http.ListenAndServe(":8080", nil)

	
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

        // Priority Check 3: Extract modern universal reindexing target webhook parameter
        if envWebhook := os.Getenv("REINDEX_WEBHOOK_URL"); envWebhook != "" {
                cfg.ReindexWebhookURL = envWebhook
        }

        // Priority Check 4: Extract legacy Jellyfin connection definitions
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

func generateDefaultArt(podcastName string, showDir string) error {
    // 1. Sanitize the name for use as a filename
    safeName := sanitizeFilename(podcastName)
    filePath := filepath.Join(showDir, fmt.Sprintf("%s.jpg", safeName))

    // 2. Skip if the artwork already exists to save processing
    if _, err := os.Stat(filePath); err == nil {
        return nil 
    }

    // 3. Create 500x500 image
    img := image.NewRGBA(image.Rect(0, 0, 500, 500))
    
    // Background color (e.g., a dark Slate-800 to match your UI)
    bgColor := color.RGBA{30, 41, 59, 255} 
    draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

    // 4. Save to disk
    f, err := os.Create(filePath)
    if err != nil {
        return err
    }
    defer f.Close()
    
    return jpeg.Encode(f, img, &jpeg.Options{Quality: 85})
}

// processFeed evaluates a show directory target, fires down retention clean sweeps, and checks for updates.
func processFeed(cfg *Config, feed PodcastFeed, startDate, endDate time.Time, protectedPaths map[string]bool) error {
        showDir := filepath.Join(cfg.LibraryDir, feed.Name)
        if err := os.MkdirAll(showDir, 0755); err != nil {
                return err
        }

        // Drop expired assets before processing network streams.
        if err := cleanOldEpisodes(showDir, cfg.RetentionDays, protectedPaths); err != nil {
                return err
        }

        fp := gofeed.NewParser()
        ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
        defer cancel()

        parsedFeed, err := fp.ParseURLWithContext(feed.URL, ctx)
        if err != nil {
                return err
        }

		// Generate artwork if it doesn't exist
    	if err := generateDefaultArt(feed.Name, showDir); err != nil {
        	logEvent("WARN", fmt.Sprintf("Could not generate artwork for %s: %v", feed.Name, err))
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

// triggerGenericReindex fires an HTTP POST request to a platform-agnostic target webhook.
func triggerGenericReindex(cfg *Config) {
        client := &http.Client{Timeout: 15 * time.Second}

        req, err := http.NewRequest("POST", cfg.ReindexWebhookURL, nil)
        if err != nil {
                logEvent("WARN", fmt.Sprintf("Failed to construct generic library re-index request: %v", err))
                return
        }

        resp, err := client.Do(req)
        if err != nil {
                logEvent("WARN", fmt.Sprintf("Universal library reindex notification failed: %v", err))
                return
        }
        defer resp.Body.Close()

        logEvent("WEBHOOK", fmt.Sprintf("Universal reindex trigger dispatched. Remote endpoint responded with Status: %s", resp.Status))
}

// triggerJellyfinScan pings Jellyfin to trigger a legacy library scan.
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
