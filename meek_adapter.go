package main

import (
	"os/exec"
	"os"
	"log"
	"fmt"
)

func logKill(p *os.Process) error {
	log.Printf("killing PID %d", p.Pid)
	err := p.Kill()
	if err != nil {
		log.Print(err)
	}
	return err
}

func runMeekClient(cmdName string, args []string) (cmd *exec.Cmd, err error) {
	//TODO: where to put meek's command line client?
	//Ellipsis allows you to pass a slice as a variadic parameter
	cmd = exec.Command(cmdName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("running meek-client command %q", cmd.Args)
	err = cmd.Start()
	if err != nil {
		return
	}
	log.Printf("meek-client started with pid %d", cmd.Process.Pid)
	err = cmd.Wait()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error waiting for Cmd", err)
		os.Exit(1)
	}
	return cmd, nil
}

func configureEnv() (error) {
	err := os.Setenv("TOR_PT_MANAGED_TRANSPORT_VER", "1")
	if err != nil {
		return err
	}
	err = os.Setenv("TOR_PT_CLIENT_TRANSPORTS", "meek")
	return nil
}

func main() {
	//TODO: put all configuration flags for PTs in a config file.

	configureEnv()

	//meek-client --url=https://meek-reflect.appspot.com/ --front=www.google.com
	cmd := "C:\\Users\\Audrey\\go\\src\\github.com\\arlolra\\meek\\meek-client\\meek-client.exe"
	args := []string{"--url=https://meek-reflect.appspot.com/", "--front=www.google.com", "--log=meek-client.log"}
	meekClientCmd, err := runMeekClient(cmd, args)
	if err != nil {
		log.Print(err)
		return
	}
	//TODO: Figure out what kind of message the client expects, and send it, because it isn't just a browser connection.
	defer logKill(meekClientCmd.Process)
}
