package http

import (
	"log"
	"net/http"
	"os/exec"
)

func generateSupportFile() {
	log.Println("Generating support file")
	cmd := exec.Command("/rcade/scripts/rcade-commands.sh", "supportfiles")

	// Run the command and wait for it to complete
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Command finished with error: %v", err)
	}

	log.Println("Support file generated")
}

var supportFileHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	generateSupportFile()

	return http.StatusNoContent, nil
})
