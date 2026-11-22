package record

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Record struct {
	Timestamp         time.Time
	Tool              string
	Version           string
	Algorithm         string
	Label             string
	Status            string
	Password          string
	Charset           string
	MinLength         int
	MaxLength         int
	Workers           int
	Attempts          uint64
	Elapsed           time.Duration
	AttemptsPerSecond float64
	SearchSpace       uint64
	HashParameters    string
	Device            string
}

var header = []string{
	"timestamp",
	"tool",
	"version",
	"algorithm",
	"label",
	"status",
	"password",
	"charset",
	"min_length",
	"max_length",
	"workers",
	"attempts",
	"elapsed_seconds",
	"attempts_per_second",
	"search_space",
	"hash_parameters",
	"device",
}

func open(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
}

func Prepare(path string) error {
	file, err := open(path)
	if err != nil {
		return err
	}

	return file.Close()
}

func Append(path string, value Record) (err error) {
	file, err := open(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	writer := csv.NewWriter(file)
	if info.Size() == 0 {
		if err = writer.Write(header); err != nil {
			return err
		}
	}

	row := []string{
		value.Timestamp.UTC().Format(time.RFC3339Nano),
		value.Tool,
		value.Version,
		value.Algorithm,
		value.Label,
		value.Status,
		value.Password,
		value.Charset,
		strconv.Itoa(value.MinLength),
		strconv.Itoa(value.MaxLength),
		strconv.Itoa(value.Workers),
		strconv.FormatUint(value.Attempts, 10),
		strconv.FormatFloat(value.Elapsed.Seconds(), 'f', -1, 64),
		strconv.FormatFloat(value.AttemptsPerSecond, 'f', -1, 64),
		strconv.FormatUint(value.SearchSpace, 10),
		value.HashParameters,
		value.Device,
	}
	if err = writer.Write(row); err != nil {
		return err
	}

	writer.Flush()
	if err = writer.Error(); err != nil {
		return fmt.Errorf("write CSV: %w", err)
	}

	return nil
}
