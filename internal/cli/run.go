package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"runtime"
	"time"

	"beza/internal/checker"
	"beza/internal/record"
	"beza/internal/search"
)

const Version = "1.0.0"

func resolveWorkers(requested int, explicit bool, memory uint64) (int, error) {
	if explicit && requested <= 0 {
		return 0, fmt.Errorf("workers must be positive")
	}

	if requested > 0 {
		return requested, nil
	}

	if memory > 0 {
		return 1, nil
	}

	return runtime.NumCPU(), nil
}

func timeoutFlag(flags *flag.FlagSet) *time.Duration {
	return flags.Duration("timeout", 0, "")
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("hashbrute", flag.ContinueOnError)
	flags.SetOutput(stderr)

	algorithm := flags.String("algorithm", "", "")
	hash := flags.String("hash", "", "")
	charsetValue := flags.String("charset", "digits", "")
	minLength := flags.Int("min-length", 1, "")
	maxLength := flags.Int("max-length", 8, "")
	requestedWorkers := flags.Int("workers", 0, "")
	timeout := timeoutFlag(flags)
	progress := flags.Duration("progress", time.Second, "")
	label := flags.String("label", "", "")
	output := flags.String("output", "", "")

	if err := flags.Parse(args); err != nil {
		return errorOut(stderr, err)
	}

	workersExplicitlySet := false
	flags.Visit(func(value *flag.Flag) {
		if value.Name == "workers" {
			workersExplicitlySet = true
		}
	})

	if *timeout < 0 || *progress < 0 {
		return errorOut(stderr, fmt.Errorf("timeout and progress must not be negative"))
	}

	target, err := checker.New(*algorithm, *hash)
	if err != nil {
		return errorOut(stderr, err)
	}

	workers, err := resolveWorkers(*requestedWorkers, workersExplicitlySet, target.MemoryKiB())
	if err != nil {
		return errorOut(stderr, err)
	}

	charset, err := search.ResolveCharset(*charsetValue)
	if err != nil {
		return errorOut(stderr, err)
	}

	cfg, err := search.NewConfig(charset, *minLength, *maxLength, workers, *progress)
	if err != nil {
		return errorOut(stderr, err)
	}

	if *output != "" {
		if err := record.Prepare(*output); err != nil {
			return errorOut(stderr, err)
		}
	}

	runCtx := ctx
	cancel := func() {}
	if *timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, *timeout)
	}
	defer cancel()

	if target.MemoryKiB() > 0 && workersExplicitlySet && workers > 1 {
		fmt.Fprintf(stderr, "Warning: estimated Argon2id memory: %d KiB\n", uint64(workers)*target.MemoryKiB())
	}

	var onProgress func(search.Snapshot)
	if *progress > 0 {
		onProgress = func(snapshot search.Snapshot) {
			fmt.Fprintf(stderr, "Progress: attempts=%d rate=%.2f\n", snapshot.Attempts, snapshot.AverageRate)
		}
	}

	result, err := search.Search(runCtx, cfg, target, onProgress)
	if err != nil {
		return errorOut(stderr, err)
	}

	if *output != "" {
		value := record.Record{
			Timestamp:         time.Now(),
			Tool:              "hashbrute",
			Version:           Version,
			Algorithm:         target.Algorithm(),
			Label:             *label,
			Status:            string(result.Status),
			Password:          result.Password,
			Charset:           charset.Name,
			MinLength:         *minLength,
			MaxLength:         *maxLength,
			Workers:           workers,
			Attempts:          result.Attempts,
			Elapsed:           result.Elapsed,
			AttemptsPerSecond: result.Rate,
			SearchSpace:       cfg.Total,
			HashParameters:    target.Parameters(),
			Device:            "cpu",
		}
		if err := record.Append(*output, value); err != nil {
			return errorOut(stderr, err)
		}
	}

	fmt.Fprintf(stdout, "Algorithm: %s\nStatus: %s\n", target.Algorithm(), result.Status)
	if result.Password != "" {
		fmt.Fprintf(stdout, "Password: %s\n", result.Password)
	}
	fmt.Fprintf(stdout, "Attempts: %d\nElapsed: %s\nRate: %.2f\nWorkers: %d\nSearch space: %d\n", result.Attempts, result.Elapsed, result.Rate, workers, cfg.Total)

	switch result.Status {
	case search.StatusFound, search.StatusExhausted:
		return 0
	case search.StatusTimeout:
		return 1
	case search.StatusCancelled:
		return 130
	default:
		return 2
	}
}

func errorOut(writer io.Writer, err error) int {
	fmt.Fprintf(writer, "Status: error\nError: %v\n", err)
	return 2
}
