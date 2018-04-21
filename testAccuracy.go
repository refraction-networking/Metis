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

func executeCurl(line string, output *os.File){
	cmd := exec.Command("curl", "-x", "127.0.0.1:8080", "-s","--connect-timeout", "5", "-m", "10", line)
	err := cmd.Run()
	if err != nil {
		fmt.Println(line)
		output.WriteString(line+": "+err.Error()+"\n")
		output.Sync()
	}
}

func executeAB(line string) string {
	//Run Apache Benchmark through Metis, attempting to connect to the Alexa top N
	output, err := exec.Command("ab", "-X", "127.0.0.1:8080", "-c", "1", "-n", "1", "http://"+line+"/").Output()
	if err != nil {
		fmt.Println("Error for line ", line, " is:", err)
		return ""
	}
	out := string(output) 
	//fmt.Println(out)
	startIdx := strings.Index(out, "Time per request:")
	endIdx := strings.Index(out, "[ms]")
	avgReqTime := out[startIdx+17:endIdx]
	avgReqTime = strings.Trim(avgReqTime, " 	")
	fmt.Println("avgTime:", avgReqTime)
	return avgReqTime
}

func main() {
	test := "ab"

	file, err := os.Open("alexa_top_100.txt")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	output, err := os.Create("alexa_top_detours.txt")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	abResults, err := os.Create("ab_results.txt")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer file.Close()
	defer output.Close()
	defer abResults.Close()

	scanner := bufio.NewScanner(file)
	x := 0
	var rtts []string
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "www.") {
			line = "www."+line
		}
		if test == "curl" {
			copies := exp(x)
			for i := 0; i < int(copies); i++ {
				executeCurl(line, output)
				time.Sleep(2 * time.Second)
			}
			x++
		} else if test == "ab" {
			rtt := executeAB(line)
			rtts = append(rtts, rtt)
		}
	}
	
	if test == "ab" {
		for i := 0; i < len(rtts); i++ {
			_, err := abResults.WriteString(rtts[i])
			if err != nil {
				fmt.Println(err)
			}
			abResults.Sync()
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

