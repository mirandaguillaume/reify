package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
	"github.com/mirandaguillaume/reify/internal/scanner"
)

// WatchOptions configures the file watcher.
type WatchOptions struct {
	SkillsDir  string
	AgentsDir  string
	OutputDir  string
	Target     string
	DebounceMs int
	EnrichMode scanner.EnrichMode
}

// WatchController manages the lifecycle of a file watcher.
type WatchController struct {
	watcher *fsnotify.Watcher
	stopCh  chan struct{}
	once    sync.Once
}

// Stop terminates the watcher and cleans up resources.
func (w *WatchController) Stop() {
	w.once.Do(func() {
		close(w.stopCh)
		w.watcher.Close()
	})
}

// FormatTimestamp returns the current time as HH:MM:SS.
func FormatTimestamp() string {
	now := time.Now()
	return fmt.Sprintf("%02d:%02d:%02d", now.Hour(), now.Minute(), now.Second())
}

// IsRelevantFile checks if a filename is a skill or agent YAML file.
func IsRelevantFile(name string) bool {
	return strings.HasSuffix(name, ".skill.yaml") || strings.HasSuffix(name, ".agent.yaml")
}

// CreateWatcher starts a file watcher that rebuilds on changes.
func CreateWatcher(opts WatchOptions) *WatchController {
	debounceMs := opts.DebounceMs
	if debounceMs == 0 {
		debounceMs = 300
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println(color.RedString("Failed to create watcher: %v", err))
		os.Exit(1)
	}

	ctrl := &WatchController{
		watcher: watcher,
		stopCh:  make(chan struct{}),
	}

	rebuild := func() {
		ts := FormatTimestamp()
		fmt.Printf("%s Rebuilding...\n", color.BlueString("[%s]", ts))
		result := RunBuild(opts.SkillsDir, opts.AgentsDir, opts.OutputDir, opts.Target, opts.EnrichMode)
		PrintBuildResult(result)
		if result.Success {
			fmt.Printf("%s Watching for changes...\n", color.GreenString("[%s]", FormatTimestamp()))
		}
	}

	// Initial build
	rebuild()

	// Start debounced watcher
	var timer *time.Timer
	var mu sync.Mutex

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					if IsRelevantFile(filepath.Base(event.Name)) {
						mu.Lock()
						if timer != nil {
							timer.Stop()
						}
						timer = time.AfterFunc(time.Duration(debounceMs)*time.Millisecond, rebuild)
						mu.Unlock()
					}
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			case <-ctrl.stopCh:
				return
			}
		}
	}()

	// Watch directories
	for _, dir := range []string{opts.SkillsDir, opts.AgentsDir} {
		if _, err := os.Stat(dir); err == nil {
			watcher.Add(dir)
		}
	}

	return ctrl
}
