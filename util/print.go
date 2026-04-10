package util

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Print formats and prints text to the command's designated standard output stream.
func Print(cmd *cobra.Command, args ...interface{}) {
	fmt.Fprint(cmd.OutOrStdout(), args...)
}

// Printf formats and prints text to the command's designated standard output stream.
func Printf(cmd *cobra.Command, format string, args ...interface{}) {
	fmt.Fprintf(cmd.OutOrStdout(), format, args...)
}

// Println prints text with a trailing newline to the command's designated standard output stream.
func Println(cmd *cobra.Command, args ...interface{}) {
	fmt.Fprintln(cmd.OutOrStdout(), args...)
}

// Warnf prints a warning message to the command's designated error output stream.
func Warnf(cmd *cobra.Command, format string, args ...interface{}) {
	fmt.Fprintf(cmd.ErrOrStderr(), format, args...)
}

// Warnln prints a warning message with a newline to the command's error output stream.
func Warnln(cmd *cobra.Command, args ...interface{}) {
	fmt.Fprintln(cmd.ErrOrStderr(), args...)
}

// Errorf prints an error message to the command's designated error output stream.
func Errorf(cmd *cobra.Command, format string, args ...interface{}) {
	fmt.Fprintf(cmd.ErrOrStderr(), format, args...)
}

// Errorln prints an error message with a newline to the command's error output stream.
func Errorln(cmd *cobra.Command, args ...interface{}) {
	fmt.Fprintln(cmd.ErrOrStderr(), args...)
}

// Successf prints a success message to the command's output stream.
func Successf(cmd *cobra.Command, format string, args ...interface{}) {
	fmt.Fprintf(cmd.OutOrStdout(), format, args...)
}

// Successln prints a success message with a newline to the command's output stream.
func Successln(cmd *cobra.Command, args ...interface{}) {
	fmt.Fprintln(cmd.OutOrStdout(), args...)
}
