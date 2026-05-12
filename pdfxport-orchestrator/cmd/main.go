package main

/**
 * PDFXport Orchestrator
 * 
 * This server acts as the "Brain" of the automation. 
 * It receives harvested data from the browser, queues it in SQLite,
 * and handles the sequential download of PDFs from the Sieval API.
 */

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"pdfxport-orchestrator/internal/client"
	"pdfxport-orchestrator/internal/queue"
	"pdfxport-orchestrator/internal/storage"
)

// Metrics tracks the performance of the current harvest session.
type Metrics struct {
	StartTime   time.Time
	LastJobTime time.Time
	TotalCount  int
	TotalBytes  int64
	mu          sync.Mutex
}

func main() {
	metrics := &Metrics{}

	// Initialize components
	store := storage.New("./output")
	q := queue.New("./jobs.db")
	
	// Create the API client (token will be updated per job)
	apiClient := client.New("https://sievalhub.sieval.com", "") 

	log.Println("🚀 PDFXport Orchestrator Ready (Dynamic Auth Enabled)")

	// =========================================================================
	// HTTP API: Listens for incoming jobs from the Chrome Extension
	// =========================================================================
	go func() {
		http.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
			// CORS headers allow the browser extension to POST data to localhost
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			var req struct {
				OrderNum   string `json:"orderNum"`
				ProjectID  int    `json:"projectId"`
				DocumentID int    `json:"documentId"`
				Polygons   []int  `json:"polygons"`
				Lang       string `json:"lang"`
				Token      string `json:"token"`
			}
			
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				return
			}

			// Validate essential data
			if len(req.Polygons) == 0 || req.Token == "" {
				w.WriteHeader(http.StatusOK)
				return
			}

			// Start the timer when the first job hits the server
			metrics.mu.Lock()
			if metrics.StartTime.IsZero() {
				metrics.StartTime = time.Now()
			}
			metrics.mu.Unlock()

			// Add job to SQLite queue. The token is stored alongside the job.
			q.AddWithToken(req.OrderNum, req.ProjectID, req.DocumentID, req.Polygons, req.Lang, req.Token)
			w.WriteHeader(http.StatusOK)
		})
		
		log.Println("📡 Listening for browser jobs on :8765")
		http.ListenAndServe(":8765", nil)
	}()

	// =========================================================================
	// PROCESSING LOOP: Sequentially processes the queue
	// =========================================================================
	for {
		// Fetch one pending job from the database
		batch, err := q.FetchBatch(1)
		if err != nil || len(batch) == 0 {
			// If idle for 10 seconds, print a session summary
			if !metrics.StartTime.IsZero() && metrics.TotalCount > 0 && time.Since(metrics.LastJobTime) > 10*time.Second {
				metrics.mu.Lock()
				duration := metrics.LastJobTime.Sub(metrics.StartTime)
				fmt.Printf("\n--- 🏁 HARVEST SUMMARY ---\n")
				fmt.Printf("📦 Total Files: %d\n", metrics.TotalCount)
				fmt.Printf("💾 Total Size:  %.2f MB\n", float64(metrics.TotalBytes)/(1024*1024))
				fmt.Printf("⏱️ Total Time:  %s\n", duration.Round(time.Second))
				fmt.Printf("🚀 Avg Speed:   %.2f s/file\n", duration.Seconds()/float64(metrics.TotalCount))
				fmt.Printf("--------------------------\n\n")
				
				// Reset session metrics
				metrics.TotalCount = 0
				metrics.TotalBytes = 0
				metrics.StartTime = time.Time{}
				metrics.mu.Unlock()
			}
			time.Sleep(1 * time.Second)
			continue
		}

		job := batch[0]
		
		// 1. Dynamic Authentication: Use the token captured from the browser
		apiClient.SetToken(job.Token)

		// 2. Generation: Request the PDF from Sieval
		data, err := apiClient.GeneratePDF(job.ProjectID, job.DocumentID, job.Polygons, job.Lang)
		if err != nil {
			log.Printf("❌ Job %d Failed: %v", job.ID, err)
			q.MarkFailed(job.ID, err.Error())
			continue
		}

		// 3. Storage: Save the file using the project number as filename
		_, _ = store.Save(job.OrderNum, job.ID, data)
		
		// 4. Cleanup: Mark the job as done in SQLite
		q.MarkDone(job.ID)

		// Update metrics
		metrics.mu.Lock()
		metrics.TotalCount++
		metrics.TotalBytes += int64(len(data))
		metrics.LastJobTime = time.Now()
		fmt.Printf("[%d] ✅ %-15s | %d KB\n", metrics.TotalCount, job.OrderNum, len(data)/1024)
		metrics.mu.Unlock()
	}
}
