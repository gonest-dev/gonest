package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

var (
	mu         sync.Mutex
	cmd        *exec.Cmd
	lastBuild  time.Time
	buildDelay = 500 * time.Millisecond
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: gonest-dev <main-file> [args...]")
		os.Exit(1)
	}

	mainFile := os.Args[1]
	appArgs := os.Args[2:]

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Initial start
	restart(mainFile, appArgs)

	// Watch directories recursively
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && !shouldIgnore(path) {
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to walk: %v", err)
	}

	fmt.Println("👀 Watching for changes...")

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				if isWatchedFile(event.Name) {
					mu.Lock()
					if time.Since(lastBuild) > buildDelay {
						lastBuild = time.Now()
						fmt.Printf("📂 Change detected: %s\n", event.Name)
						go restart(mainFile, appArgs)
					}
					mu.Unlock()
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

func shouldIgnore(path string) bool {
	ignoreList := []string{".git", "vendor", ".gemini", "_tools", "node_modules", ".vscode", "bin"}
	for _, ignore := range ignoreList {
		if strings.Contains(path, ignore) {
			return true
		}
	}
	return false
}

func isWatchedFile(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".go" || ext == ".env" || name == "go.mod" || name == "go.sum"
}

func restart(mainFile string, appArgs []string) {
	mu.Lock()
	defer mu.Unlock()

	// 1. Kill current process if running
	if cmd != nil && cmd.Process != nil {
		fmt.Println("🛑 Stopping current process...")
		// Send SIGTERM for graceful shutdown
		err := cmd.Process.Signal(os.Interrupt)
		if err != nil {
			cmd.Process.Kill()
		}
		cmd.Wait() // Wait for it to actually stop
	}

	// 2. Build
	fmt.Printf("🔨 Building %s...\n", mainFile)
	buildCmd := exec.Command("go", "build", "-o", "bin/app.exe", mainFile)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fmt.Printf("❌ Build failed: %v\n", err)
		return
	}

	// 3. Start
	fmt.Println("🚀 Starting application...")
	cmd = exec.Command("./bin/app.exe", appArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "GONEST_DEV_MODE=true")

	if err := cmd.Start(); err != nil {
		fmt.Printf("❌ Failed to start application: %v\n", err)
		return
	}
}
