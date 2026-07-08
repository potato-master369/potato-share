package main

import (
	"bufio"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	_ "net/http/pprof"
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

var extensionIcons = make(map[string]string)
var loadpath = "/home/ebayan"
var templates = make(map[string]*template.Template)

func loadTemplates() error {
	files := map[string]string{
		"grid":            "static/grid.html",
		"list":            "static/list.html",
		"preview-img":     "static/preview-img.html",
		"preview-video":   "static/preview-video.html",
		"preview-invalid": "static/preview-invalid.html",
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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(32 << 20)
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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
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

func main() {
	args := os.Args
	port := 8000
	err := error(nil)
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

	http.HandleFunc("/list/", ServeFileBrowser)
	http.HandleFunc("/download/", ServeDownload)
	http.HandleFunc("/upload", ServeUpload)
	http.HandleFunc("/mkdir", ServeMkdir)
	http.HandleFunc("/grid/", ServeFileBrowserGrid)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Just straight up serve the file. http.ServeFile handles the
		// content-type headers and streams the bytes efficiently.
		http.ServeFile(w, r, "./static/index.html")
	})

	fs := http.FileServer(http.Dir(loadpath))
	http.Handle("/static/", http.StripPrefix("/static", fs))
	http.HandleFunc("/preview/", ServePreview)

	server := &http.Server{
		Addr:           fmt.Sprintf("0.0.0.0:%d", port),
		Handler:        nil,     // Uses the default mux handlers we registered above
		ReadTimeout:    0,       // 0 means NO timeout (perfect for slow/huge uploads)
		WriteTimeout:   0,       // 0 means NO timeout for downloading huge files
		MaxHeaderBytes: 1 << 20, // 1MB max header size (standard)
	}

	fmt.Println("Server starting on http://0.0.0.0:" + strconv.Itoa(port) + "...")

	// This binds the server to all network interfaces on port 8000
	err = server.ListenAndServe()
	if err != nil {
		log.Fatal("ListenAndServe Error: ", err)
	}
}
