package docfactory

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"encoding/json/v2"
)

// AlienationErr indicates that forking a new process is not possible
// because TES could not find out its own FS path (this is unlikely)
var AlienationErr error = errors.New("don't know who I am")

// ForkedDoc represents a Document processed by forked subprocess of this service
type ForkedDoc struct {
	cmd        *exec.Cmd
	metadata   map[string]string
	textStream io.Reader
	cancel     context.CancelFunc
	origin     string
	path       string
	log        *slog.Logger
}

func (df *DocFactory) NewDocFromForkedProcessPath(path, origin string) (*ForkedDoc, error) {
	if len(df.executable) == 0 {
		return nil, AlienationErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	cmd := exec.CommandContext(ctx, df.executable, path)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	buf := bufio.NewReader(stdout)
	df.log.Debug("Starting subprocess", "origin", origin)
	cmd.WaitDelay = time.Minute
	err = cmd.Start()
	if err != nil {
		cancel()
		return nil, err
	}
	df.log.Debug("Subprocess started", "pid", cmd.Process.Pid, "origin", origin, "cmd", cmd.Args)
	doc := &ForkedDoc{cmd: cmd, textStream: buf, cancel: cancel, origin: origin, log: df.log}
	// Read one line to get the metadata
	firstLine := readFirstLine(buf)
	metadata := make(map[string]string)
	if err := json.Unmarshal(firstLine, &metadata); err != nil {
		df.log.Error("Malformed input encountered when reading metadata from subprocess", "err", err, "origin", origin, "input", firstLine)
		return nil, fmt.Errorf("when unmarshalling JSON encoded metadata from subprocess: %w", err)
	}
	doc.metadata = metadata
	df.log.Debug("Finished reading metadata from subprocess", "origin", origin, "pid", cmd.Process.Pid)
	return doc, err
}

// NewDocFromForkedProcess creates a Document whose content and metadata is being extracted by a forked subprocess
func (df *DocFactory) NewDocFromForkedProcess(r io.Reader, origin string) (*ForkedDoc, error) {
	if len(df.executable) == 0 {
		return nil, AlienationErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	cmd := exec.CommandContext(ctx, df.executable, "-")
	cmd.Stdin = r
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	buf := bufio.NewReader(stdout)
	df.log.Debug("Starting subprocess", "origin", origin)
	cmd.WaitDelay = time.Minute
	err = cmd.Start()
	if err != nil {
		cancel()
		return nil, err
	}
	df.log.Info("Subprocess started", "pid", cmd.Process.Pid, "origin", origin, "cmd", cmd.Args)
	doc := &ForkedDoc{cmd: cmd, textStream: buf, cancel: cancel, origin: origin, log: df.log}
	// Read one line to get the metadata
	firstLine := readFirstLine(buf)
	metadata := make(map[string]string)
	if err := json.Unmarshal(firstLine, &metadata); err != nil {
		df.log.Error("Malformed input encountered when reading metadata from subprocess", "err", err, "origin", origin, "input", firstLine)
		return nil, err
	}
	doc.metadata = metadata
	df.log.Debug("Finished reading metadata from subprocess", "origin", origin)
	return doc, err
}

func readFirstLine(r *bufio.Reader) []byte {
	result, _ := r.ReadBytes('\n')
	return result
}

func (d *ForkedDoc) Pages() int {
	return -1
}

func (d *ForkedDoc) Data() *[]byte {
	return nil
}

func (d *ForkedDoc) Path() string {
	return d.path
}

func (d *ForkedDoc) Text(i int) (string, bool) {
	panic("not allowed")
}

// StreamText may only be invoked once!
func (d *ForkedDoc) StreamText(w io.Writer) error {
	written, err := io.Copy(w, d.textStream)
	if err != nil {
		return err
	}
	d.log.Debug("Finished reading from subprocess", "bytes", written, "origin", d.origin)
	err = d.cmd.Wait()
	if err != nil {
		return err
	}
	d.log.Info("Subprocess finished", "state", d.cmd.ProcessState.String(), "origin", d.origin)
	return nil
}

func (d *ForkedDoc) MetadataMap() map[string]string {
	return d.metadata
}

func (d *ForkedDoc) Close() {
	d.cancel()
	_ = d.cmd.Cancel()
}
