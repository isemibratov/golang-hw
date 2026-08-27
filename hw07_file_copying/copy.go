package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

var (
	// ErrUnsupportedFile indicates that the source is not a regular file.
	ErrUnsupportedFile = errors.New("unsupported file")
	// ErrOffsetExceedsFileSize indicates that the offset is beyond the source EOF.
	ErrOffsetExceedsFileSize = errors.New("offset exceeds file size")
)

// Copy copies part of a regular file and writes progress to the standard output.
func Copy(fromPath, toPath string, offset, limit int64) error {
	return copyFile(fromPath, toPath, offset, limit, os.Stdout)
}

func copyFile(fromPath, toPath string, offset, limit int64, progressOutput io.Writer) (resultErr error) {
	if offset < 0 {
		return errors.New("offset must not be negative")
	}
	if limit < 0 {
		return errors.New("limit must not be negative")
	}

	source, err := os.Open(fromPath)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer func() { _ = source.Close() }()

	sourceInfo, err := source.Stat()
	if err != nil {
		return fmt.Errorf("stat source file: %w", err)
	}
	if !sourceInfo.Mode().IsRegular() {
		return ErrUnsupportedFile
	}
	if offset > sourceInfo.Size() {
		return ErrOffsetExceedsFileSize
	}
	if _, err = source.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("seek source file: %w", err)
	}

	bytesToCopy := sourceInfo.Size() - offset
	if limit > 0 && limit < bytesToCopy {
		bytesToCopy = limit
	}

	destination, err := os.OpenFile(toPath, os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return fmt.Errorf("open destination file: %w", err)
	}
	defer func() {
		if closeErr := destination.Close(); resultErr == nil && closeErr != nil {
			resultErr = fmt.Errorf("close destination file: %w", closeErr)
		}
	}()

	destinationInfo, err := destination.Stat()
	if err != nil {
		return fmt.Errorf("stat destination file: %w", err)
	}
	if os.SameFile(sourceInfo, destinationInfo) {
		return errors.New("source and destination are the same file")
	}
	if err = destination.Truncate(0); err != nil {
		return fmt.Errorf("truncate destination file: %w", err)
	}

	progress := progressWriter{
		Writer:      destination,
		output:      progressOutput,
		total:       bytesToCopy,
		lastPercent: -1,
	}
	progress.report(0)

	if _, err = io.CopyN(&progress, source, bytesToCopy); err != nil {
		_, _ = fmt.Fprintln(progressOutput)
		return fmt.Errorf("copy file contents: %w", err)
	}
	progress.report(100)
	_, _ = fmt.Fprintln(progressOutput)

	return nil
}

type progressWriter struct {
	io.Writer
	output      io.Writer
	total       int64
	written     int64
	lastPercent int
}

func (writer *progressWriter) Write(data []byte) (int, error) {
	written, err := writer.Writer.Write(data)
	writer.written += int64(written)
	if err == nil {
		percentage := 100
		if writer.total > 0 {
			percentage = int(float64(writer.written) * 100 / float64(writer.total))
		}
		writer.report(percentage)
	}

	return written, err
}

func (writer *progressWriter) report(percentage int) {
	if percentage == writer.lastPercent {
		return
	}

	_, _ = fmt.Fprintf(writer.output, "\rCopying: %3d%%", percentage)
	writer.lastPercent = percentage
}
