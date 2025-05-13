package dht

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	logStderr   = false
	logMu       sync.Mutex
	logFileOnce sync.Once
	logFile     *os.File

	metricsMu sync.Mutex
	metrics   = struct {
		SentRPCs    int
		RecvRPCs    int
		FailedRPCs  int
		LastPrinted time.Time
	}{LastPrinted: time.Now()}
)

func initLogFile() {
	f, err := os.OpenFile("dht.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Failed to open log file:", err)
		return
	}
	logFile = f
}

func logToFile(entry string) {
	if logStderr {
		fmt.Fprintln(os.Stderr, entry)
	}
	logFileOnce.Do(initLogFile)
	if logFile != nil {
		logFile.WriteString(entry + "\n")
	}
}

func LogRPC(direction string, msg Message, addr string) {
	logMu.Lock()
	defer logMu.Unlock()
	timestamp := time.Now().Format("15:04:05.000")
	entry := fmt.Sprintf("[%s] %s %s -> %s [%s]", timestamp, direction, msg.Type, addr, msg.Key)
	logToFile(entry)

	metricsMu.Lock()
	if direction == "SEND" {
		metrics.SentRPCs++
	} else if direction == "RECV" {
		metrics.RecvRPCs++
	}
	printMetricsSummary()
	metricsMu.Unlock()
}

func LogRPCFailure(msg string) {
	logMu.Lock()
	logToFile("[ERROR] " + msg)
	logMu.Unlock()

	metricsMu.Lock()
	metrics.FailedRPCs++
	printMetricsSummary()
	metricsMu.Unlock()
}

func printMetricsSummary() {
	now := time.Now()
	if now.Sub(metrics.LastPrinted) >= 10*time.Second {
		summary := fmt.Sprintf("[METRICS] Sent: %d, Recv: %d, Fail: %d",
			metrics.SentRPCs, metrics.RecvRPCs, metrics.FailedRPCs)
		logToFile(summary)
		metrics.LastPrinted = now
	}
}

func LogRoutingTableSize(rt *RoutingTable) {
	total := 0
	for _, b := range rt.Buckets {
		total += len(b.Nodes)
	}
	entry := fmt.Sprintf("[ROUTING] Total known peers: %d", total)
	logToFile(entry)
}

func LogTrace(msg Message, phase string) {
	timestamp := time.Now().Format("15:04:05.000")
	trace := map[string]any{
		"timestamp": timestamp,
		"phase":     phase,
		"type":      msg.Type,
		"from":      msg.From.Address,
		"key":       msg.Key,
		"results":   len(msg.Results),
	}
	data, _ := json.Marshal(trace)
	logToFile(string(data))
}
