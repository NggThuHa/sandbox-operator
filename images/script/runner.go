package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
)

type CheckRequest struct {
	Command string `json:"command"`
}

type CheckResponse struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

func handleCheck(w http.ResponseWriter, r *http.Request) {
	var req CheckRequest
	json.NewDecoder(r.Body).Decode(&req)

	// Chạy trực tiếp lệnh kiểm tra trong Pod
	cmd := exec.Command("bash", "-c", req.Command)
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	json.NewEncoder(w).Encode(CheckResponse{
		ExitCode: exitCode,
		Stdout:   string(out),
	})
}

func main() {
	http.HandleFunc("/run-check", handleCheck)
	http.ListenAndServe(":8090", nil)
}
