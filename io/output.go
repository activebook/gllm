package io

import (
	"bufio"
	"fmt"
	"os"
)

// Output defines the interface for output formatting.
type Output interface {
	Writeln(args ...interface{})
	Writef(format string, args ...interface{})
	Write(args ...interface{})
	Close()
}

type StdOutput struct {
}

func NewStdOutput() *StdOutput {
	return &StdOutput{}
}

func (r *StdOutput) Writef(format string, args ...interface{}) {
	// Print the output to the console
	fmt.Printf(format, args...)
}

func (r *StdOutput) Writeln(args ...interface{}) {
	// Print the output to the console
	fmt.Println(args...)
}

func (r *StdOutput) Write(args ...interface{}) {
	// Print the output to the console
	fmt.Print(args...)
}

func (r *StdOutput) Close() {
	// Do nothing
}

// FileOutput is a renderer that writes output to a file
type FileOutput struct {
	file   *os.File
	writer *bufio.Writer
}

// NewFileOutput creates a new instance of FileOutput
func NewFileOutput(filename string) (*FileOutput, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	writer := bufio.NewWriter(file)

	return &FileOutput{
		file:   file,
		writer: writer,
	}, nil
}

// Writef writes formatted output to the file
func (fr *FileOutput) Writef(format string, args ...interface{}) {
	if fr.writer != nil {
		fmt.Fprintf(fr.writer, format, args...)
	}
}

func (fr *FileOutput) Write(args ...interface{}) {
	if fr.writer != nil {
		fmt.Fprint(fr.writer, args...)
	}
}

func (fr *FileOutput) Writeln(args ...interface{}) {
	if fr.writer != nil {
		fmt.Fprintln(fr.writer, args...)
	}
}

// Close closes the file renderer and its underlying file
func (fr *FileOutput) Close() {
	if fr.writer != nil {
		fr.writer.Flush()
		fr.writer = nil
	}

	if fr.file != nil {
		fr.file.Close()
		fr.file = nil
	}
}

// GetFilename returns the name of the file being written to
func (fr *FileOutput) GetFilename() string {
	if fr.file != nil {
		return fr.file.Name()
	}
	return ""
}
