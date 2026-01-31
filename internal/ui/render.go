package ui

import (
	"bufio"
	"fmt"
	"os"
)

type Render interface {
	Writeln(args ...interface{})
	Writef(format string, args ...interface{})
	Write(args ...interface{})
}

type StdRenderer struct {
}

func NewStdRenderer() *StdRenderer {
	return &StdRenderer{}
}

func (r *StdRenderer) Writef(format string, args ...interface{}) {
	// Print the output to the console
	fmt.Printf(format, args...)
}

func (r *StdRenderer) Writeln(args ...interface{}) {
	// Print the output to the console
	fmt.Println(args...)
}

func (r *StdRenderer) Write(args ...interface{}) {
	// Print the output to the console
	fmt.Print(args...)
}

// FileRenderer is a renderer that writes output to a file
type FileRenderer struct {
	file   *os.File
	writer *bufio.Writer
}

// NewFileRenderer creates a new instance of FileRenderer
func NewFileRenderer(filename string) (*FileRenderer, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	writer := bufio.NewWriter(file)

	return &FileRenderer{
		file:   file,
		writer: writer,
	}, nil
}

// Writef writes formatted output to the file
func (fr *FileRenderer) Writef(format string, args ...interface{}) {
	if fr.writer != nil {
		fmt.Fprintf(fr.writer, format, args...)
		// Everytime flush is too intense
		//fr.writer.Flush() // Ensure immediate write to file
	}
}

func (fr *FileRenderer) Write(args ...interface{}) {
	if fr.writer != nil {
		fmt.Fprint(fr.writer, args...)
		// Everytime flush is too intense
		//fr.writer.Flush() // Ensure immediate write to file
	}
}

func (fr *FileRenderer) Writeln(args ...interface{}) {
	if fr.writer != nil {
		fmt.Fprintln(fr.writer, args...)
		// Everytime flush is too intense
		//fr.writer.Flush() // Ensure immediate write to file
	}
}

// Close closes the file renderer and its underlying file
func (fr *FileRenderer) Close() error {
	if fr.writer != nil {
		fr.writer.Flush()
		fr.writer = nil
	}

	if fr.file != nil {
		err := fr.file.Close()
		fr.file = nil
		return err
	}

	return nil
}

// GetFilename returns the name of the file being written to
func (fr *FileRenderer) GetFilename() string {
	if fr.file != nil {
		return fr.file.Name()
	}
	return ""
}
