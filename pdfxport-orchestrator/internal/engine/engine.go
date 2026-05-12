package engine

import (
	"log"
	"sync"

	"pdfxport-orchestrator/internal/queue"
)

type Engine struct {
	Workers int
}

func New(workers int) *Engine {
	return &Engine{Workers: workers}
}

func (e *Engine) Run(q *queue.Queue, handler func(queue.Job)) {

	jobs, err := q.FetchBatch(100)
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	ch := make(chan queue.Job)

	for i := 0; i < e.Workers; i++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for job := range ch {
				log.Printf("👷 Worker %d → Job %d", id, job.ID)
				handler(job)
			}
		}(i)
	}

	for _, job := range jobs {
		ch <- job
	}

	close(ch)
	wg.Wait()
}
