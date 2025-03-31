// File: main.go
package main

import (
	// Use the module path you defined in 'go mod init' + '/cmd'
	"github.com/activebook/gllm/cmd"
)

func main() {
	//test.TestQwQ()
	//return
	cmd.Execute()
}
