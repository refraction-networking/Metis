package main

import (
	"fmt"
	"os"
	"log"
	"bufio"
	"strings"
	"os/exec"
)

func main() {
	file, err := os.Open("log/detour.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "www.") {
			line = "www."+line
		}
		fmt.Println(line)
		cmd := exec.Command("curl", "-s","--connect-timeout 5", "-m 10", line)
		err := cmd.Run()
		fmt.Println(err)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
