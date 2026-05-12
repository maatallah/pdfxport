package engine

import (
	"log"

	"pdfxport-orchestrator/internal/client"
	"pdfxport-orchestrator/internal/queue"
	"pdfxport-orchestrator/internal/storage"
)

func Handler(
	c *client.Client,
	store *storage.Store,
	q *queue.Queue,
) func(queue.Job) {

	return func(job queue.Job) {

		log.Println("🚀 Processing job:", job.ID)

		pdfBytes, err := c.GeneratePDF(
			job.ProjectID,
			job.DocumentID,
			job.Polygons,
			job.Lang,
		)

		if err != nil {

			log.Println("❌ Failed job:", job.ID)
			log.Println("❌ ERROR:", err)

			q.MarkFailed(job.ID, err.Error())

			return
		}

		savedPath, err := store.Save(job.OrderNum, job.ID, pdfBytes)

		if err != nil {

			log.Println("❌ Storage failed:", err)

			q.MarkFailed(job.ID, err.Error())

			return
		}

		log.Println("💾 Saved:", savedPath)
		q.MarkDone(job.ID)

		log.Println("✅ Completed job:", job.ID)
	}
}
