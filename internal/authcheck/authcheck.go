package authcheck

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Result struct {
	Strategy  string `json:"strategy"`
	Available bool   `json:"available"`
	Detail    string `json:"detail,omitempty"`
}

func Check(strategy string) Result {
	s := strings.TrimSpace(strategy)
	if s == "" || s == "inherit" {
		return Result{Strategy: s, Available: true}
	}

	switch s {
	case "ssh-agent":
		if os.Getenv("SSH_AUTH_SOCK") == "" {
			return Result{
				Strategy:  s,
				Available: false,
				Detail:    "SSH_AUTH_SOCK is not set",
			}
		}
		return Result{Strategy: s, Available: true}
	case "gh":
		return checkCommand(s, "gh", "auth", "status")
	case "glab":
		return checkCommand(s, "glab", "auth", "status")
	default:
		return Result{
			Strategy:  s,
			Available: false,
			Detail:    "unknown auth strategy",
		}
	}
}

func checkCommand(strategy string, bin string, args ...string) Result {
	if _, err := exec.LookPath(bin); err != nil {
		return Result{
			Strategy:  strategy,
			Available: false,
			Detail:    fmt.Sprintf("%s not found", bin),
		}
	}

	cmd := exec.Command(bin, args...)
	if err := cmd.Run(); err != nil {
		return Result{
			Strategy:  strategy,
			Available: false,
			Detail:    fmt.Sprintf("%s %s failed", bin, strings.Join(args, " ")),
		}
	}
	return Result{Strategy: strategy, Available: true}
}
