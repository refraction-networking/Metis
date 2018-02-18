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
	file, err := os.Open("nd_log/detour.txt")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	output, err := os.Create("nd_log/detour_results.txt")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer file.Close()
	defer output.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "www.") {
			line = "www."+line
		}
		cmd := exec.Command("curl", "-s","--connect-timeout", "5", "-m", "10", line)
		err := cmd.Run()
		if err == nil {
			fmt.Println(line)
			output.WriteString(line+"\n")
			output.Sync()
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

/* Ended at:
 * www.w3schools.com
www.11st.co.kr
www.milliyet.com.tr
www.google.dz
*/
