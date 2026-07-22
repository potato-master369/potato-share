package main

import (
	"bufio"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

type FileRow struct {
	Name        string
	Link        string
	PreviewLink string
	Icon        string
	ModTime     string
	Size        string
}

type PageData struct {
	Host        string
	CurrentPath string
	Files       []FileRow
}

type PasswordRow struct {
	Password string
	Perms    string
}

type AdminPageData struct {
	Passwords []PasswordRow
}

type DataRow struct {
	Password string
}

type DataRow2 struct {
	uuid       int64
	ip         string
	expiration time.Time
}

var extensionIcons = make(map[string]string)
var loadpath = "/home/ebayan"
var templates = make(map[string]*template.Template)

// define stuff for DB
var insertPwd *sql.Stmt
var insertSession *sql.Stmt
var db *sql.DB

func loadTemplates() error {
	files := map[string]string{
		"grid":            "static/grid.html",
		"list":            "static/list.html",
		"preview-img":     "static/preview-img.html",
		"preview-video":   "static/preview-video.html",
		"preview-invalid": "static/preview-invalid.html",
		"admin":           "static/admin.html",
	}

	for key, path := range files {
		t, err := template.ParseFiles(path)
		if err != nil {
			return fmt.Errorf("error parsing template %s: %w", path, err)
		}
		templates[key] = t
	}

	return nil
}

func formatFileSize(bytes int64) string {
	if bytes < 0 {
		return "Invalid size"
	}
	if bytes == 0 {
		return "0 B"
	}

	units := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	size := float64(bytes)
	base := 1024.0
	exp := int(math.Floor(math.Log(size) / math.Log(base)))
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.2f %s", size/math.Pow(base, float64(exp)), units[exp])
}

func loadIconsConfig(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			ext := strings.TrimSpace(parts[0])
			icon := strings.TrimSpace(parts[1])
			extensionIcons[ext] = icon
		}
	}

	return scanner.Err()
}

func checkPassword(password string) (bool, string, error) {
	var stored, perms string
	err := db.QueryRow("SELECT string, perms FROM passwords WHERE string = ?", password).Scan(&stored, &perms)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return stored == password, perms, nil
}

func generateSessionToken() (string, error) {
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func setSessionCookie(w http.ResponseWriter, token string) {
	expires := time.Now().Add(2 * time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   7200,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ServeLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		http.ServeFile(w, r, "./static/login.html")
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	password := r.FormValue("password")
	if password == "" {
		http.Error(w, "Password required", http.StatusBadRequest)
		return
	}

	ok, perms, err := checkPassword(password)
	if err != nil {
		log.Printf("password check failed: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token, err := generateSessionToken()
	if err != nil {
		log.Printf("failed to generate session token: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = newSession(token, r.RemoteAddr, perms)
	if err != nil {
		log.Printf("failed to save session: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, token)
	http.Redirect(w, r, "/list/", http.StatusSeeOther)
}

func getSessionToken(r *http.Request) (string, error) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

func getSessionPerms(r *http.Request) (string, error) {
	token, err := getSessionToken(r)
	if err != nil {
		return "", err
	}

	var perms string
	err = db.QueryRow("SELECT perms FROM sessions WHERE token = ?", token).Scan(&perms)
	if err != nil {
		return "", err
	}
	return perms, nil
}

func getAuthError(w http.ResponseWriter, r *http.Request, err error) {
	if err == http.ErrNoCookie {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	} else {
		http.Error(w, "Bad Request", http.StatusBadRequest)
	}
}

func AuthHandler(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := getSessionToken(r)
		if err != nil {
			getAuthError(w, r, err)
			return
		}

		var expiresAt time.Time
		err = db.QueryRow("SELECT expiration FROM sessions WHERE token = ?", token).Scan(&expiresAt)
		if err != nil || time.Now().After(expiresAt) {
			http.Error(w, "Unauthorized or session expired", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func getIconForFile(filename string) string {
	ext := filepath.Ext(filename)
	ext = strings.TrimPrefix(ext, ".")

	if icon, exists := extensionIcons[ext]; exists {
		return icon
	} else if ext == "DIR" {
		return "fa-solid fa-folder"
	}
	return "fa-solid fa-file" // Default icon
}

func ServeFileBrowserGrid(w http.ResponseWriter, r *http.Request) {
	currentPath := strings.TrimPrefix(r.URL.Path, "/grid")
	entries, err := os.ReadDir(loadpath + currentPath)
	if err != nil {
		http.Error(w, "Unable to read directory", http.StatusInternalServerError)
		return
	}

	fileRows := []FileRow{}

	if currentPath != "/" && currentPath != "" {
		parentPath := filepath.Dir(strings.TrimSuffix(currentPath, "/"))

		if parentPath != "/" {
			parentPath += "/"
		}

		fileRows = append(fileRows, FileRow{
			Name:    "../ (Up one directory)",
			Link:    "/grid" + parentPath,
			Icon:    "fa-solid fa-folder-open",
			ModTime: "-",
			Size:    "-",
		})
	}
	hostname, err := os.Hostname()

	for _, e := range entries {
		filename := e.Name()
		f, err := os.Open(loadpath + currentPath + filename)
		mtime := time.Now()
		if err != nil {
			fmt.Println("Error opening file:", err)
		}

		fi, err := f.Stat()
		if err != nil {
			fmt.Println("Error getting file info:", err)
		}

		mtime = fi.ModTime()

		size := formatFileSize(fi.Size())

		link := "/download" + currentPath + filename
		link = filepath.Clean(link)

		previewlink := "/preview" + currentPath + filename
		previewlink = filepath.Clean(previewlink)

		icon := getIconForFile(filename)

		if e.IsDir() {
			filename += "/"
			size = "-"                  // Directories don't traditionally show a byte size
			icon = "fa-solid fa-folder" // Override icon for folders
			// If it's a directory, clicking it should browse into it, not download it!
			link = "/grid" + currentPath + "/" + e.Name() + "/"
			link = filepath.Clean(link) + "/"
			icon = "fa-solid fa-folder"
		}

		fileRows = append(fileRows, FileRow{
			PreviewLink: previewlink,
			Name:        filename,
			Link:        link,
			Icon:        icon,
			ModTime:     mtime.Format("2006-01-02 15:04:05"),
			Size:        size,
		})
	}

	data := PageData{
		Host:        hostname,
		CurrentPath: currentPath,
		Files:       fileRows,
	}

	tmpl := templates["grid"]
	if tmpl == nil {
		http.Error(w, "Internal Server Error: Missing or broken template", http.StatusInternalServerError)
		fmt.Println("Missing parsed template: grid")
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println("Template execution error:", err)
	}
}

func ServePreview(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/preview")
	localPath := loadpath + filePath

	fi, err := os.Stat(localPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	ext := strings.ToLower(filepath.Ext(fi.Name()))
	if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".bmp" || ext == ".webp" {
		data := struct {
			Name    string
			URL     string
			Size    string
			ModTime string
		}{
			Name:    fi.Name(),
			URL:     "/static" + filePath,
			Size:    formatFileSize(fi.Size()),
			ModTime: fi.ModTime().Format("2006-01-02 15:04:05"),
		}

		tmpl := templates["preview-img"]
		if tmpl == nil {
			http.Error(w, "Internal Server Error: Missing or broken template", http.StatusInternalServerError)
			fmt.Println("Missing parsed template: preview-img")
			return
		}

		err = tmpl.Execute(w, data)
		if err != nil {
			fmt.Println("Template execution error:", err)
		}
	} else if ext == ".mp4" || ext == ".webm" || ext == ".ogg" || ext == ".mkv" {
		data := struct {
			Name    string
			URL     string
			Size    string
			ModTime string
		}{
			Name:    fi.Name(),
			URL:     "/static" + filePath,
			Size:    formatFileSize(fi.Size()),
			ModTime: fi.ModTime().Format("2006-01-02 15:04:05"),
		}

		tmpl := templates["preview-video"]
		if tmpl == nil {
			http.Error(w, "Internal Server Error: Missing or broken template", http.StatusInternalServerError)
			fmt.Println("Missing parsed template: preview-video")
			return
		}

		err = tmpl.Execute(w, data)
		if err != nil {
			fmt.Println("Template execution error:", err)
		}
	} else {
		// For invalid
		data := struct {
			Name    string
			URL     string
			Size    string
			ModTime string
		}{
			Name:    fi.Name(),
			URL:     "/static" + filePath,
			Size:    formatFileSize(fi.Size()),
			ModTime: fi.ModTime().Format("2006-01-02 15:04:05"),
		}

		tmpl := templates["preview-invalid"]
		if tmpl == nil {
			http.Error(w, "Internal Server Error: Missing or broken template", http.StatusInternalServerError)
			fmt.Println("Missing parsed template: preview-invalid")
			return
		}

		err = tmpl.Execute(w, data)
		if err != nil {
			fmt.Println("Template execution error:", err)
		}
	}
}

func ServeFileBrowser(w http.ResponseWriter, r *http.Request) {
	currentPath := strings.TrimPrefix(r.URL.Path, "/list")
	entries, err := os.ReadDir(loadpath + currentPath)
	if err != nil {
		http.Error(w, "Unable to read directory", http.StatusInternalServerError)
		return
	}

	fileRows := []FileRow{}

	if currentPath != "/" && currentPath != "" {
		parentPath := filepath.Dir(strings.TrimSuffix(currentPath, "/"))

		if parentPath != "/" {
			parentPath += "/"
		}

		fileRows = append(fileRows, FileRow{
			Name:    "../ (Up one directory)",
			Link:    "/list" + parentPath,
			Icon:    "fa-solid fa-folder-open",
			ModTime: "-",
			Size:    "-",
		})
	}
	hostname, err := os.Hostname()

	for _, e := range entries {
		filename := e.Name()
		f, err := os.Open(loadpath + currentPath + filename)
		mtime := time.Now()
		if err != nil {
			fmt.Println("Error opening file:", err)
		}

		fi, err := f.Stat()
		if err != nil {
			fmt.Println("Error getting file info:", err)
		}

		mtime = fi.ModTime()

		size := formatFileSize(fi.Size())

		link := "/download" + currentPath + filename
		link = filepath.Clean(link)

		previewlink := "/preview" + currentPath + filename
		previewlink = filepath.Clean(previewlink)

		icon := getIconForFile(filename)

		if e.IsDir() {
			filename += "/"
			size = "-"                  // Directories don't traditionally show a byte size
			icon = "fa-solid fa-folder" // Override icon for folders
			// If it's a directory, clicking it should browse into it, not download it!
			link = "/list" + currentPath + "/" + e.Name() + "/"
			link = filepath.Clean(link) + "/"
			icon = "fa-solid fa-folder"
		}

		fileRows = append(fileRows, FileRow{
			PreviewLink: previewlink,
			Name:        filename,
			Link:        link,
			Icon:        icon,
			ModTime:     mtime.Format("2006-01-02 15:04:05"),
			Size:        size,
		})
	}

	data := PageData{
		Host:        hostname,
		CurrentPath: currentPath,
		Files:       fileRows,
	}

	tmpl := templates["list"]
	if tmpl == nil {
		http.Error(w, "Internal Server Error: Missing or broken template", http.StatusInternalServerError)
		fmt.Println("Missing parsed template: list")
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println("Template execution error:", err)
	}
}

func ServeDownload(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/download")
	localPath := loadpath + filePath

	fi, err := os.Stat(localPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fi.Name()))
	http.ServeFile(w, r, localPath)
}

func ServeUpload(w http.ResponseWriter, r *http.Request) {
	perms, err := getSessionPerms(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if perms == "ro" {
		http.Error(w, "Forbidden: read-only users cannot upload", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err = r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, "File too large or bad request", http.StatusBadRequest)
		return
	}

	currentPath := r.FormValue("path")

	file, header, err := r.FormFile("myFile")
	if err != nil {
		http.Error(w, "Error retrieving file from form", http.StatusBadRequest)
		return
	}
	defer file.Close()

	destPath := ""
	targetDir := filepath.Clean(loadpath + currentPath)
	if _, err := os.Stat(filepath.Join(targetDir, header.Filename)); os.IsNotExist(err) {
		destPath = filepath.Join(targetDir, header.Filename)
	} else {
		destPath = filepath.Join(targetDir, fmt.Sprintf("%s_%d", header.Filename, time.Now().Unix()))
	}
	out, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "Unable to save file on server", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		http.Error(w, "Failed to save file contents", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/list"+currentPath, http.StatusSeeOther)
}

func ServeMkdir(w http.ResponseWriter, r *http.Request) {
	perms, err := getSessionPerms(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if perms == "ro" {
		http.Error(w, "Forbidden: read-only users cannot create directories", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err = r.ParseForm()
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	currentPath := r.FormValue("path")
	folderName := r.FormValue("folderName")
	returnTo := r.FormValue("returnTo")
	if folderName == "" {
		http.Error(w, "Folder name cannot be empty", http.StatusBadRequest)
		return
	}

	newDirPath := filepath.Join(loadpath, currentPath, folderName)
	err = os.Mkdir(newDirPath, 0755)
	if err != nil {
		http.Error(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, returnTo+currentPath, http.StatusSeeOther)
}

func ensureSessionPermsColumn() error {
	rows, err := db.Query("PRAGMA table_info(sessions)")
	if err != nil {
		return err
	}
	defer rows.Close()

	hasPerms := false
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "perms" {
			hasPerms = true
			break
		}
	}

	if !hasPerms {
		_, err = db.Exec("ALTER TABLE sessions ADD COLUMN perms TEXT NOT NULL DEFAULT 'user'")
		if err != nil {
			return err
		}
	}
	return nil
}

func cleanupExpiredSessions() error {
	_, err := db.Exec("DELETE FROM sessions WHERE expiration <= ?", time.Now())
	return err
}

func newSession(token, ip, perms string) error {
	if err := cleanupExpiredSessions(); err != nil {
		log.Printf("session cleanup failed before insert: %v", err)
	}

	expiration := time.Now().Add(2 * time.Hour) // Set expiration to 2 hours from now
	_, err := insertSession.Exec(token, ip, expiration, perms)
	if err != nil {
		return fmt.Errorf("failed to insert session: %w", err)
	}
	return nil
}

func AdminMiddleware(db *sql.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil {
			if err == http.ErrNoCookie {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		var expiresAt time.Time
		var perms string
		err = db.QueryRow("SELECT expiration, perms FROM sessions WHERE token = ?", cookie.Value).Scan(&expiresAt, &perms)
		if err != nil {
			http.Error(w, "Unauthorized or session expired", http.StatusUnauthorized)
			return
		}
		if time.Now().After(expiresAt) {
			if _, delErr := db.Exec("DELETE FROM sessions WHERE token = ?", cookie.Value); delErr != nil {
				log.Printf("failed to delete expired session: %v", delErr)
			}
			http.Error(w, "Unauthorized or session expired", http.StatusUnauthorized)
			return
		}

		if perms != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

func ServeAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.Query("SELECT string, perms FROM passwords ORDER BY string")
	if err != nil {
		log.Printf("failed to query passwords: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	passwords := []PasswordRow{}
	for rows.Next() {
		var pw PasswordRow
		if err := rows.Scan(&pw.Password, &pw.Perms); err != nil {
			log.Printf("failed to scan password row: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		passwords = append(passwords, pw)
	}

	data := AdminPageData{Passwords: passwords}

	tmpl := templates["admin"]
	if tmpl == nil {
		http.Error(w, "Internal Server Error: Missing or broken template", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("admin template execution error: %v", err)
	}
}

func ServePasswordCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	password := strings.TrimSpace(r.FormValue("passwd"))
	perms := strings.TrimSpace(r.FormValue("perms"))
	if password == "" {
		http.Error(w, "Password required", http.StatusBadRequest)
		return
	}
	if perms == "" {
		perms = "user"
	}

	_, err = db.Exec("INSERT OR IGNORE INTO passwords(string, perms) VALUES(?, ?)", password, perms)
	if err != nil {
		log.Printf("failed to create password: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func ServePasswordDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	password := strings.TrimPrefix(r.URL.Path, "/passwd-delete/")
	if password == "" {
		http.Error(w, "Password not specified", http.StatusBadRequest)
		return
	}

	_, err := db.Exec("DELETE FROM passwords WHERE string = ?", password)
	if err != nil {
		log.Printf("failed to delete password: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func AuthMiddleware(db *sql.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil {
			if err == http.ErrNoCookie {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		var expiresAt time.Time
		err = db.QueryRow("SELECT expiration FROM sessions WHERE token = ?", cookie.Value).Scan(&expiresAt)
		if err != nil || time.Now().After(expiresAt) {
			http.Error(w, "Unauthorized or session expired", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func main() {
	args := os.Args
	port := 8000
	var err error
	db, err = sql.Open("sqlite3", "./potato-share.db")
	if err != nil {
		log.Fatal("Error opening database:", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS passwords (
		string TEXT PRIMARY KEY,
		perms TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		ip TEXT NOT NULL,
		expiration DATETIME NOT NULL,
		perms TEXT NOT NULL DEFAULT 'user'
	);`
	if _, err := db.Exec(createTable); err != nil {
		log.Fatalf("Error creating tables: %v", err)
	}

	if err := ensureSessionPermsColumn(); err != nil {
		log.Fatalf("Error ensuring session perms column: %v", err)
	}

	insertPwd, err = db.Prepare("INSERT OR IGNORE INTO passwords(string, perms) VALUES(?, ?)")
	if err != nil {
		log.Fatalf("Error preparing password insert: %v", err)
	}
	insertSession, err = db.Prepare("INSERT INTO sessions(token, ip, expiration, perms) VALUES(?, ?, ?, ?)")
	if err != nil {
		log.Fatalf("Error preparing session insert: %v", err)
	}

	_, err = insertPwd.Exec("potato", "admin")
	if err != nil {
		log.Fatalf("Error inserting password: %v", err)
	}

	if len(args) > 1 {
		loadpath = args[1]
	}
	if len(args) > 2 {
		port, err = strconv.Atoi(args[2])
		if err != nil {
			fmt.Println("Error parsing port:", err)
			return
		}
	}

	err = loadIconsConfig("ext-fa.txt")
	if err != nil {
		fmt.Println("Error loading icons config:", err)
		return
	}

	err = loadTemplates()
	if err != nil {
		fmt.Println("Error loading templates:", err)
		return
	}

	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := cleanupExpiredSessions(); err != nil {
				log.Printf("session cleanup failed: %v", err)
			}
		}
	}()

	http.HandleFunc("/list/", AuthMiddleware(db, ServeFileBrowser))
	http.HandleFunc("/download/", AuthMiddleware(db, ServeDownload))
	http.HandleFunc("/upload", AuthMiddleware(db, ServeUpload))
	http.HandleFunc("/mkdir", AuthMiddleware(db, ServeMkdir))
	http.HandleFunc("/grid/", AuthMiddleware(db, ServeFileBrowserGrid))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/index.html")
	})

	fs := http.FileServer(http.Dir(loadpath))
	http.Handle("/static/", AuthHandler(db, http.StripPrefix("/static", fs)))
	http.HandleFunc("/preview/", AuthMiddleware(db, ServePreview))
	http.HandleFunc("/login", ServeLogin)
	http.HandleFunc("/admin", AdminMiddleware(db, ServeAdmin))
	http.HandleFunc("/passwd-create", AdminMiddleware(db, ServePasswordCreate))
	http.HandleFunc("/passwd-delete/", AdminMiddleware(db, ServePasswordDelete))

	server := &http.Server{
		Addr:           fmt.Sprintf("0.0.0.0:%d", port),
		Handler:        nil,
		ReadTimeout:    0,
		WriteTimeout:   0,
		MaxHeaderBytes: 1 << 20,
	}

	fmt.Println("Server starting on http://0.0.0.0:" + strconv.Itoa(port) + "...")

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal("ListenAndServe Error: ", err)
	}
}
