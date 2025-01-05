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
}

// NewDocFromForkedProcess creates a Document whose content and metadata is being extracted by a forked subprocess
func NewDocFromForkedProcess(r io.Reader) (Document, error) {
	me, err := os.Executable()
	if err != nil {
		logger.Error("Could not find out who I am", "err", err)
		return nil, err
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Minute))
	cmd := exec.CommandContext(ctx, me, "-")
	cmd.Stdin = r
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	scanner := bufio.NewScanner(stdout)
	logger.Info("Starting subprocess")
	cmd.WaitDelay = time.Minute
	cmd.Start()
	logger.Debug("Subprocess started", "pid", cmd.Process.Pid)
	doc := &ForkedDoc{cmd: cmd, textStream: stdout, cancel: cancel}
	// Read one line to get the metadata
	if scanner.Scan() {
		metadataJson := scanner.Text()
		metadata := make(map[string]string)
		json.Unmarshal([]byte(metadataJson), &metadata)
		doc.metadata = metadata
	}
	logger.Debug("Finished reading metadata from subprocess")
	return doc, err
}

// StreamText may only be invoked once!
func (d *ForkedDoc) StreamText(w io.Writer) {
	written, err := io.Copy(w, d.textStream)
	if err != nil {
		logger.Error("Reading from subprocess after", "bytes", written, "err", err)
		return
	}
	logger.Debug("Finished reading from subprocess", "bytes", written)
	err = d.cmd.Wait()
	if err != nil {
		logger.Error("Error waiting for subprocess to finish", "err", err)
	}
	logger.Info("Subprocess finished", "state", d.cmd.ProcessState.String())
}

func (d *ForkedDoc) ProcessPages(w io.Writer, process func(pageText string, pageIndex int, w io.Writer, pdfData *[]byte)) {
	d.StreamText(w)
}

func (d *ForkedDoc) MetadataMap() map[string]string {
	return d.metadata
}

func (d *ForkedDoc) Close() {
	// the subprocesses stdout should already be closed...
	d.textStream.Close()
	d.cancel()
	d.cmd.Cancel()
}