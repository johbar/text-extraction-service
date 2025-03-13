package main

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/go-json-experiment/json"
)

// ForkedDoc represents a Document processed by forked subprocess of this service
type ForkedDoc struct {
	cmd        *exec.Cmd
	metadata   map[string]string
	textStream io.Reader
	cancel     context.CancelFunc
	origin     string
}

// NewDocFromForkedProcess creates a Document whose content and metadata is being extracted by a forked subprocess
func NewDocFromForkedProcess(r io.Reader, origin string) (*ForkedDoc, error) {
	me, err := os.Executable()
	if err != nil {
		logger.Error("Could not find out who I am", "err", err, "origin", origin)
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	cmd := exec.CommandContext(ctx, me, "-")
	cmd.Stdin = r
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	buf := bufio.NewReader(stdout)
	logger.Debug("Starting subprocess", "origin", origin)
	cmd.WaitDelay = time.Minute
	err = cmd.Start()
	if err != nil {
		cancel()
		return nil, err
	}
	logger.Info("Subprocess started", "pid", cmd.Process.Pid, "origin", origin)
	doc := &ForkedDoc{cmd: cmd, textStream: buf, cancel: cancel, origin: origin}
	// Read one line to get the metadata
	firstLine := readFirstLine(buf)
	metadata := make(map[string]string)
	if err := json.Unmarshal(firstLine, &metadata); err != nil {
		logger.Error("Malformed input encountered when reading metadata from subprocess", "err", err, "origin", origin, "input", firstLine)
		return nil, err
	}
	doc.metadata = metadata
	logger.Debug("Finished reading metadata from subprocess", "origin", origin)
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

func (d *ForkedDoc) Text(i int) (string, bool) {
	panic("not allowed")
}

// StreamText may only be invoked once!
func (d *ForkedDoc) StreamText(w io.Writer) error {
	written, err := io.Copy(w, d.textStream)
	if err != nil {
		return err
	}
	logger.Debug("Finished reading from subprocess", "bytes", written, "origin", d.origin)
	err = d.cmd.Wait()
	if err != nil {
		return err
	}
	logger.Info("Subprocess finished", "state", d.cmd.ProcessState.String(), "origin", d.origin)
	return nil
}

func (d *ForkedDoc) MetadataMap() map[string]string {
	return d.metadata
}

func (d *ForkedDoc) Close() {
	d.cancel()
	_ = d.cmd.Cancel()
}
