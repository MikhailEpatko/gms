// Команда gms показывает текущий деплой каждого окружения проектов.
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

// runGlab запускает настоящий glab и возвращает его stdout. При ошибке в неё попадает stderr glab,
// сведённый в одну строку, чтобы он помещался в строку ошибки пары.
func runGlab(args ...string) ([]byte, error) {
	out, err := exec.Command("glab", args...).Output()
	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		// glab оформляет часть ошибок баннером «ERROR» с отступами и переносами строк.
		msg := strings.Join(strings.Fields(string(ee.Stderr)), " ")
		msg = strings.TrimPrefix(msg, "ERROR ")
		msg = strings.TrimPrefix(msg, "glab: ")
		if msg != "" {
			return nil, fmt.Errorf("glab: %s", msg)
		}
	}
	return out, err
}
