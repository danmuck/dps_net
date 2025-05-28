package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func (c *Client) logsMenu(args []string) error {
	reader := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println(`

Logs:
  clear     – delete all logs files
  compress  – gzip each log file
  back      – return to main menu

  `)
		fmt.Print("logs> ")
		if !reader.Scan() {
			return nil
		}
		cmd := strings.TrimSpace(reader.Text())
		switch cmd {
		case "clear":
			if err := c.clearLogs(nil); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
			initLogs()
		case "compress":
			if err := c.compressLogs(nil); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
			initLogs()
		case "back", "exit":
			return nil
		default:
			fmt.Println("unknown logs command; choose clear, compress, or back")
		}
	}
}

// should prob not do full clear and compress and then reinit the dir
func initLogs() {
	const dir = "logs"

	// 1) ensure logs/ exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("could not create %s directory: %v", dir, err)
	}

	// 2) count existing files to pick next index
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("could not read %s directory: %v", dir, err)
	}
	n := len(entries)

	// 3) open new log file
	filePath := filepath.Join(dir, fmt.Sprintf("net_%d.log", n))
	f, err := os.OpenFile(filePath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0o666,
	)
	if err != nil {
		log.Fatalf("failed to open log file %s: %v", filePath, err)
	}

	// 4) send logs to f (and stderr if you like)
	// log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.SetOutput(f)

	// include date, time, microseconds and file/line
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
}

// clearLogs removes the entire logs/ directory.
func (c *Client) clearLogs(args []string) error {
	if err := os.RemoveAll("logs"); err != nil {
		return LogError(fmt.Errorf("failed to delete logs/: %w", err))
	}
	fmt.Println("logs/ directory deleted")
	return nil
}

// compressLogs gzips every regular file under logs/, then removes the originals.
func (c *Client) compressLogs(args []string) error {
	dir := "logs"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return LogError(fmt.Errorf("failed to read %s: %w", dir, err))
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		inPath := filepath.Join(dir, e.Name())
		outPath := inPath + ".gz"

		inFile, err := os.Open(inPath)
		if err != nil {
			return LogError(fmt.Errorf("open %s: %w", inPath, err))
		}
		defer inFile.Close()

		outFile, err := os.Create(outPath)
		if err != nil {
			return LogError(fmt.Errorf("create %s: %w", outPath, err))
		}
		gw := gzip.NewWriter(outFile)
		gw.Name = e.Name()
		entryInfo, err := e.Info()
		if err != nil {
			return LogError(fmt.Errorf("fileinfo %s: %w", outPath, err))
		}
		gw.ModTime = entryInfo.ModTime()

		if _, err := io.Copy(gw, inFile); err != nil {
			gw.Close()
			outFile.Close()
			return LogError(fmt.Errorf("compress %s: %w", inPath, err))
		}
		gw.Close()
		outFile.Close()

		// remove the original
		if err := os.Remove(inPath); err != nil {
			return LogError(fmt.Errorf("remove original %s: %w", inPath, err))
		}

		fmt.Printf("+ %s → %s\n", inPath, outPath)
	}

	fmt.Println("all logs compressed")
	return nil
}
