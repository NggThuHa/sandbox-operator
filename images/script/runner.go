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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

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

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(CheckResponse{
		ExitCode: exitCode,
		Stdout:   string(out),
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func main() {
	http.HandleFunc("/run-check", handleCheck)
	if err := http.ListenAndServe(":8090", nil); err != nil {
		panic(err)
	}
}
