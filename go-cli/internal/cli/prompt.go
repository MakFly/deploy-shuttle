package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var stdinReader *bufio.Reader

func getStdinReader() *bufio.Reader {
	if stdinReader == nil {
		stdinReader = bufio.NewReader(os.Stdin)
	}
	return stdinReader
}

// promptString asks a question and returns the answer. If empty, returns defaultVal.
func promptString(question string, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", question, defaultVal)
	} else {
		fmt.Printf("%s: ", question)
	}
	reader := getStdinReader()
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

// promptInt asks for an integer. Returns defaultVal if empty.
func promptInt(question string, defaultVal int) int {
	fmt.Printf("%s [%d]: ", question, defaultVal)
	reader := getStdinReader()
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(line)
	if err != nil {
		fmt.Printf("  Invalid number, using default %d\n", defaultVal)
		return defaultVal
	}
	return n
}

// promptChoice asks user to pick from options. Returns the chosen option.
func promptChoice(question string, options []string, defaultIdx int) string {
	fmt.Printf("%s\n", question)
	for i, opt := range options {
		marker := "  "
		if i == defaultIdx {
			marker = "* "
		}
		fmt.Printf("  %s%d) %s\n", marker, i+1, opt)
	}
	fmt.Printf("Choice [%d]: ", defaultIdx+1)
	reader := getStdinReader()
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return options[defaultIdx]
	}
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > len(options) {
		fmt.Printf("  Invalid choice, using default: %s\n", options[defaultIdx])
		return options[defaultIdx]
	}
	return options[n-1]
}

// promptConfirm asks a yes/no question. Returns bool.
func promptConfirm(question string, defaultYes bool) bool {
	suffix := "[y/N]"
	if defaultYes {
		suffix = "[Y/n]"
	}
	fmt.Printf("%s %s: ", question, suffix)
	reader := getStdinReader()
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return defaultYes
	}
	return line == "y" || line == "yes"
}
