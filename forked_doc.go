package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"time"
)

// ForkedDoc represents a Document processed by forked subprocess of this service
type ForkedDoc struct {
	cmd        *exec.Cmd
	metadata   map[string]string
	textStream io.ReadCloser
	cancel     context.CancelFunc
	origin     *string
}

// NewDocFromForkedProcess creates a Document whose content and metadata is being extracted by a forked subprocess
func NewDocFromForkedProcess(r io.Reader, origin *string) (*ForkedDoc, error) {
	me, err := os.Executable()
	if err != nil {
		logger.Error("Could not find out who I am", "err", err, "origin", origin)
		return nil, err
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Minute))
	cmd := exec.CommandContext(ctx, me, "-")
	cmd.Stdin = r
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	scanner := bufio.NewScanner(stdout)
	logger.Debug("Starting subprocess", "origin", origin)
	cmd.WaitDelay = time.Minute
	err = cmd.Start()
	if err != nil {
		cancel()
		return nil, err
	}
	logger.Info("Subprocess started", "pid", cmd.Process.Pid, "origin", origin)
	doc := &ForkedDoc{cmd: cmd, textStream: stdout, cancel: cancel, origin: origin}
	// Read one line to get the metadata
	if scanner.Scan() {
		metadataJson := scanner.Text()
		metadata := make(map[string]string)
		if err := json.Unmarshal([]byte(metadataJson), &metadata); err != nil {
			logger.Error("Malformed input encountered when reading metadata")
			return nil, err
		}
		doc.metadata = metadata
	}
	logger.Debug("Finished reading metadata from subprocess", "origin", origin)
	return doc, err
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
	// the subprocesses stdout should already be closed...
	d.textStream.Close()
	d.cancel()
	_ = d.cmd.Cancel()
}
