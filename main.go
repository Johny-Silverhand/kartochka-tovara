package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

//go:embed web/*
var webFS embed.FS

const version = "0.1.1"
const listenAddr = "127.0.0.1:8765"

func dataRoot() string {
	if d := os.Getenv("KARTOCHKA_DATA"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "kartochka-data")
	}
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("APPDATA")
		if base == "" {
			base = home
		}
		return filepath.Join(base, "VictimokLabs", "KartochkaTovara")
	default:
		return filepath.Join(home, ".local", "share", "VictimokLabs", "KartochkaTovara")
	}
}

func main() {
	store, err := OpenStore(dataRoot())
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"ok":      true,
			"version": version,
			"brand":   "Victimok Labs",
			"data":    store.Root(),
		})
	})

	mux.HandleFunc("/api/cards", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, store.List())
		case http.MethodPost:
			var body struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "bad json", 400)
				return
			}
			c, err := store.Create(strings.TrimSpace(body.Name), strings.TrimSpace(body.Description))
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			writeJSON(w, c)
		default:
			http.Error(w, "method", 405)
		}
	})

	mux.HandleFunc("/api/cards/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/cards/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 || parts[0] == "" {
			http.NotFound(w, r)
			return
		}
		id := parts[0]

		if len(parts) >= 2 && parts[1] == "photo" {
			if r.Method == http.MethodPost && len(parts) == 2 {
				if err := r.ParseMultipartForm(12 << 20); err != nil {
					http.Error(w, "multipart", 400)
					return
				}
				f, hdr, err := r.FormFile("photo")
				if err != nil {
					http.Error(w, "photo required", 400)
					return
				}
				defer f.Close()
				data, err := io.ReadAll(io.LimitReader(f, 10<<20))
				if err != nil {
					http.Error(w, "read", 500)
					return
				}
				ext := strings.ToLower(filepath.Ext(hdr.Filename))
				switch ext {
				case ".jpg", ".jpeg", ".png", ".webp", ".gif":
				default:
					ext = ".jpg"
				}
				c, err := store.AddPhoto(id, ext, data)
				if err != nil {
					code := 400
					if err.Error() == "not found" {
						code = 404
					}
					http.Error(w, err.Error(), code)
					return
				}
				writeJSON(w, c)
				return
			}
			if r.Method == http.MethodDelete && len(parts) == 3 {
				var idx int
				if _, err := fmt.Sscanf(parts[2], "%d", &idx); err != nil {
					http.Error(w, "bad index", 400)
					return
				}
				c, err := store.RemovePhoto(id, idx)
				if err != nil {
					http.Error(w, err.Error(), 404)
					return
				}
				writeJSON(w, c)
				return
			}
		}

		if len(parts) != 1 {
			http.NotFound(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet:
			c, ok := store.Get(id)
			if !ok {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, c)
		case http.MethodPut:
			var body struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "bad json", 400)
				return
			}
			c, err := store.Update(id, strings.TrimSpace(body.Name), strings.TrimSpace(body.Description))
			if err != nil {
				http.Error(w, err.Error(), 404)
				return
			}
			writeJSON(w, c)
		case http.MethodDelete:
			if err := store.Delete(id); err != nil {
				http.Error(w, err.Error(), 404)
				return
			}
			writeJSON(w, map[string]bool{"ok": true})
		default:
			http.Error(w, "method", 405)
		}
	})

	mux.HandleFunc("/api/photos/", func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(strings.TrimPrefix(r.URL.Path, "/api/photos/"))
		if name == "." || name == "/" || name == "" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, store.PhotoPath(name))
	})

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal(err)
	}
	url := "http://" + listenAddr + "/"
	fmt.Printf("Карточка товара v%s — Victimok Labs\n", version)
	fmt.Printf("Данные: %s\n", store.Root())
	fmt.Printf("Открыто: %s\n", url)
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = openBrowser(url)
	}()
	log.Fatal(http.Serve(ln, mux))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
