package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/MikhailEpatko/gms/internal/gms"
)

func main() {
	os.Exit(gms.Run(os.Args[1:], gms.Deps{
		Glab:     runGlab,
		Getenv:   os.Getenv,
		HomeDir:  os.UserHomeDir,
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
		Location: time.Local,
	}))
}

func runGlab(args ...string) ([]byte, error) {
	out, err := exec.Command("glab", args...).Output()
	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		if msg := oneLineGlabError(ee.Stderr); msg != "" {
			return nil, fmt.Errorf("glab: %s", msg)
		}
	}
	return out, err
}

func oneLineGlabError(stderr []byte) string {
	msg := strings.Join(strings.Fields(string(stderr)), " ")
	msg = strings.TrimPrefix(msg, "ERROR ")
	return strings.TrimPrefix(msg, "glab: ")
}
