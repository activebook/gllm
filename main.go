// File: main.go
package main

import (
	// Use the module path you defined in 'go mod init' + '/cmd'
	"os"

	"github.com/activebook/gllm/cmd"
	"github.com/activebook/gllm/test"
)

func main() {

	test.TestOpenai()
	os.Exit(0)
	cmd.Execute()
}
