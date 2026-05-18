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
	"os"
	"sync"
	"time"

	"pdfxport-orchestrator/internal/client"
	"pdfxport-orchestrator/internal/queue"
	"pdfxport-orchestrator/internal/storage"
	"pdfxport-orchestrator/internal/utils"

	"github.com/getlantern/systray"
)

type Metrics struct {
	StartTime   time.Time
	LastJobTime time.Time
	TotalCount  int
	TotalBytes  int64
	mu          sync.Mutex
}

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(utils.IconData)
	systray.SetTitle("PDFXport Moissonneuse")
	systray.SetTooltip("Moissonneuse PDFXport - En attente...")
	
	// Create Menu Items
	mStatus := systray.AddMenuItem("🌾 Statut: En écoute (Port 8765)", "Le serveur est actif")
	mStatus.Disable()
	
	systray.AddSeparator()
	
	mReset := systray.AddMenuItem("🧹 Vider le Grenier (Réinitialiser)", "Supprimer jobs.db pour autoriser les ré-extractions")
	mQuit := systray.AddMenuItem("🚪 Quitter", "Arrêter le serveur")

	metrics := &Metrics{}
	store := storage.New("./output")
	q := queue.New("./jobs.db")
	apiClient := client.New("https://sievalhub.sieval.com", "")

	log.Println("🌾 Moissonneuse PDFXport Prête. En attente du blé...")

	// Listen for menu actions
	go func() {
		for {
			select {
			case <-mReset.ClickedCh:
				q.Close() // Close DB before deletion
				os.Remove("./jobs.db")
				log.Println("🧹 Grenier vidé (jobs.db supprimé).")
				systray.SetTooltip("Grenier réinitialisé.")
				// Re-open queue
				q = queue.New("./jobs.db")
			case <-mQuit.ClickedCh:
				systray.Quit()
			}
		}
	}()

	// Start HTTP Server
	go func() {
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
			}
			metrics.mu.Unlock()

			q.AddWithToken(req.OrderNum, req.ProjectID, req.DocumentID, req.Polygons, req.Lang, req.Token)
			w.WriteHeader(http.StatusOK)
		})
		http.ListenAndServe(":8765", nil)
	}()

	// Processing Loop
	go func() {
		for {
			batch, err := q.FetchBatch(1)
			if err != nil || len(batch) == 0 {
				if !metrics.StartTime.IsZero() && metrics.TotalCount > 0 && time.Since(metrics.LastJobTime) > 10*time.Second {
					metrics.mu.Lock()
					duration := metrics.LastJobTime.Sub(metrics.StartTime)
					log.Printf("🏁 RÉSUMÉ: %d épis récoltés en %v", metrics.TotalCount, duration.Round(time.Second))
					metrics.TotalCount = 0
					metrics.TotalBytes = 0
					metrics.StartTime = time.Time{}
					metrics.mu.Unlock()
					systray.SetTooltip("Dernière moisson terminée.")
				}
				time.Sleep(1 * time.Second)
				continue
			}

			job := batch[0]
			apiClient.SetToken(job.Token)

			systray.SetTooltip(fmt.Sprintf("🚜 Récolte en cours : %s", job.OrderNum))
			data, err := apiClient.GeneratePDF(job.ProjectID, job.DocumentID, job.Polygons, job.Lang)
			if err != nil {
				log.Printf("❌ Erreur Job %d: %v", job.ID, err)
				q.MarkFailed(job.ID, err.Error())
				continue
			}

			_, _ = store.Save(job.OrderNum, job.ID, data)
			q.MarkDone(job.ID)

			metrics.mu.Lock()
			metrics.TotalCount++
			metrics.TotalBytes += int64(len(data))
			metrics.LastJobTime = time.Now()
			metrics.mu.Unlock()
			systray.SetTooltip(fmt.Sprintf("🌾 %d récoltés (Dernier: %s)", metrics.TotalCount, job.OrderNum))
		}
	}()
}

func onExit() {
	// Cleanup if needed
}
