package search

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"beza/internal/checker"
)

type Status string

const (
	StatusFound     Status = "found"
	StatusExhausted Status = "exhausted"
	StatusTimeout   Status = "timeout"
	StatusCancelled Status = "cancelled"
)

type Snapshot struct {
	Attempts     uint64
	Elapsed      time.Duration
	AverageRate  float64
	Progress     float64
	ETA          time.Duration
	ETAAvailable bool
}

type Result struct {
	Status   Status
	Password string
	Attempts uint64
	Elapsed  time.Duration
	Rate     float64
}

func Search(ctx context.Context, cfg Config, target checker.Checker, onProgress func(Snapshot)) (Result, error) {
	start := time.Now()
	var attempts atomic.Uint64

	if err := ctx.Err(); err != nil {
		return resultForContext(err, start, 0), nil
	}

	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan Job)
	found := make(chan string, 1)

	var producer sync.WaitGroup
	var workers sync.WaitGroup

	producer.Add(1)
	go func() {
		defer producer.Done()
		defer close(jobs)

		for _, job := range Jobs(cfg) {
			select {
			case jobs <- job:
			case <-workCtx.Done():
				return
			}
		}
	}()

	for range cfg.Workers {
		workers.Add(1)

		go func() {
			defer workers.Done()

			storage := make([]byte, cfg.MaxLength)
			var local uint64
			defer func() {
				attempts.Add(local)
			}()

			for {
				select {
				case <-workCtx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}

					cursor, err := NewCursor(cfg.Charset.Symbols, job, storage)
					if err != nil {
						return
					}

					for {
						if workCtx.Err() != nil {
							return
						}

						local++
						if local == 4096 {
							attempts.Add(local)
							local = 0
						}

						if target.Match(cursor.Value()) {
							password := string(append([]byte(nil), cursor.Value()...))
							select {
							case found <- password:
							default:
							}
							cancel()
							return
						}

						if !cursor.Next() {
							break
						}
					}
				}
			}
		}()
	}

	var stopProgress chan struct{}
	if onProgress != nil && cfg.ProgressInterval > 0 {
		stopProgress = make(chan struct{})

		go func() {
			ticker := time.NewTicker(cfg.ProgressInterval)
			defer ticker.Stop()

			for {
				select {
				case <-stopProgress:
					return
				case <-ticker.C:
					attemptCount := attempts.Load()
					elapsed := time.Since(start)
					snapshot := Snapshot{
						Attempts: attemptCount,
						Elapsed:  elapsed,
						Progress: float64(attemptCount) / float64(cfg.Total),
					}

					if elapsed > 0 {
						snapshot.AverageRate = float64(attemptCount) / elapsed.Seconds()
						if snapshot.AverageRate > 0 && attemptCount < cfg.Total {
							snapshot.ETAAvailable = true
							snapshot.ETA = time.Duration(float64(cfg.Total-attemptCount) / snapshot.AverageRate * float64(time.Second))
						}
					}

					onProgress(snapshot)
				}
			}
		}()
	}

	workers.Wait()
	producer.Wait()
	if stopProgress != nil {
		close(stopProgress)
	}

	attemptCount := attempts.Load()
	elapsed := time.Since(start)
	result := Result{Attempts: attemptCount, Elapsed: elapsed}
	if elapsed > 0 {
		result.Rate = float64(attemptCount) / elapsed.Seconds()
	}

	select {
	case password := <-found:
		result.Status = StatusFound
		result.Password = password
		return result, nil
	default:
	}

	if err := ctx.Err(); err != nil {
		return resultForContext(err, start, attemptCount), nil
	}

	result.Status = StatusExhausted
	return result, nil
}

func resultForContext(err error, start time.Time, attempts uint64) Result {
	result := Result{Attempts: attempts, Elapsed: time.Since(start)}
	if result.Elapsed > 0 {
		result.Rate = float64(attempts) / result.Elapsed.Seconds()
	}

	if err == context.DeadlineExceeded {
		result.Status = StatusTimeout
	} else {
		result.Status = StatusCancelled
	}

	return result
}
