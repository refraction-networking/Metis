package main

import (
	"fmt"
	"os"
	"log"
	"bufio"
	"strings"
	"os/exec"
	"math"
	"time"
)

func exp(x int) float64 {
	return 100.0*math.Exp(-0.2*float64(x))
}

func main() {
	file, err := os.Open("alexa_top.txt")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	output, err := os.Create("alexa_top_detours.txt")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer file.Close()
	defer output.Close()

	scanner := bufio.NewScanner(file)
	x := 0
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "www.") {
			line = "www."+line
		}
		copies := exp(x)
		for i := 0; i < int(copies); i++ {
			cmd := exec.Command("curl", "-s","--connect-timeout", "5", "-m", "10", line)
			err := cmd.Run()
			if err != nil {
				fmt.Println(line)
				output.WriteString(line+": "+err.Error()+"\n")
				output.Sync()
			}
			time.Sleep(2 * time.Second)
		}
		x++
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

