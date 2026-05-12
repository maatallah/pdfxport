package main

/**
 * Moissonneuse PDFXport (Harvester)
 * 
 * Ce serveur orchestre la récolte des PDF. 
 * Il reçoit les données, les stocke dans le "grenier" (SQLite),
 * et traite le "blé" (fichiers) de manière séquentielle.
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

type Metrics struct {
	StartTime   time.Time
	LastJobTime time.Time
	TotalCount  int
	TotalBytes  int64
	mu          sync.Mutex
}

func main() {
	metrics := &Metrics{}

	store := storage.New("./output")
	q := queue.New("./jobs.db")
	apiClient := client.New("https://sievalhub.sieval.com", "") 

	log.Println("🌾 Moissonneuse PDFXport Prête. En attente du blé...")

	go func() {
		// Point de contrôle santé
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "OK")
		})

		http.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
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

			if len(req.Polygons) == 0 || req.Token == "" {
				w.WriteHeader(http.StatusOK)
				return
			}

			metrics.mu.Lock()
			if metrics.StartTime.IsZero() {
				metrics.StartTime = time.Now()
				log.Println("🚜 Démarrage de la moissonneuse...")
			}
			metrics.mu.Unlock()

			q.AddWithToken(req.OrderNum, req.ProjectID, req.DocumentID, req.Polygons, req.Lang, req.Token)
			w.WriteHeader(http.StatusOK)
		})
		http.ListenAndServe(":8765", nil)
	}()

	for {
		batch, err := q.FetchBatch(1)
		if err != nil || len(batch) == 0 {
			if !metrics.StartTime.IsZero() && metrics.TotalCount > 0 && time.Since(metrics.LastJobTime) > 10*time.Second {
				metrics.mu.Lock()
				duration := metrics.LastJobTime.Sub(metrics.StartTime)
				fmt.Printf("\n--- 🏁 RÉSUMÉ DE LA MOISSON ---\n")
				fmt.Printf("🌾 Épis récoltés:  %d\n", metrics.TotalCount)
				fmt.Printf("📦 Poids du grain: %.2f MB\n", float64(metrics.TotalBytes)/(1024*1024))
				fmt.Printf("⏱️ Durée totale:   %s\n", duration.Round(time.Second))
				fmt.Printf("🚀 Rendement:      %.2f s/épi\n", duration.Seconds()/float64(metrics.TotalCount))
				fmt.Printf("-------------------------------\n\n")
				metrics.TotalCount = 0
				metrics.TotalBytes = 0
				metrics.StartTime = time.Time{}
				metrics.mu.Unlock()
			}
			time.Sleep(1 * time.Second)
			continue
		}

		job := batch[0]
		apiClient.SetToken(job.Token)

		data, err := apiClient.GeneratePDF(job.ProjectID, job.DocumentID, job.Polygons, job.Lang)
		if err != nil {
			log.Printf("❌ Grain gâté (Job %d): %v", job.ID, err)
			q.MarkFailed(job.ID, err.Error())
			continue
		}

		_, _ = store.Save(job.OrderNum, job.ID, data)
		q.MarkDone(job.ID)

		metrics.mu.Lock()
		metrics.TotalCount++
		metrics.TotalBytes += int64(len(data))
		metrics.LastJobTime = time.Now()
		fmt.Printf("[%d] 🌾 %-15s | %d KB (Mis au grenier)\n", metrics.TotalCount, job.OrderNum, len(data)/1024)
		metrics.mu.Unlock()
	}
}
