package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to Sweet-Bytes Application!")
}

func main() {
	// Render автоматично призначає порт через змінну оточення PORT
	port := os.Getenv("PORT")
	if port == "" {
		port = "10000" // Значення за замовчуванням для локального запуску
	}

	http.HandleFunc("/", handler)

	fmt.Printf("Server is starting on port %s...\n", port)
	err := http.ListenAndServe(":" + port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
