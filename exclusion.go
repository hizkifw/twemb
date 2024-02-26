package main

import (
	"encoding/json"
	"log"
	"os"
)

var (
	excludedUsers = map[string]bool{}
)

const (
	exclusionsFile = "exclusions.json"
)

func loadExclusions() {
	if _, err := os.Stat(exclusionsFile); os.IsNotExist(err) {
		return
	}

	file, err := os.Open(exclusionsFile)
	if err != nil {
		log.Println("Error opening exclusions file:", err)
		return
	}

	defer file.Close()

	decoder := json.NewDecoder(file)
	if err = decoder.Decode(&excludedUsers); err != nil {
		log.Println("Error decoding exclusions file:", err)
		return
	}

	log.Printf("Loaded %d excluded users from %s", len(excludedUsers), exclusionsFile)
}

func saveExclusions() {
	// Save to a temp file first
	file, err := os.CreateTemp(".", "ex")
	if err != nil {
		log.Println("Error creating temp file:", err)
		return
	}

	encoder := json.NewEncoder(file)
	if err = encoder.Encode(excludedUsers); err != nil {
		log.Println("Error encoding exclusions file:", err)
		return
	}

	file.Close()

	// Replace the old file with the new one
	if err = os.Rename(file.Name(), exclusionsFile); err != nil {
		log.Println("Error renaming temp file:", err)
		return
	}

	log.Printf("Saved %d excluded users to %s", len(excludedUsers), exclusionsFile)
}

func isUserExcluded(userID string) bool {
	_, ok := excludedUsers[userID]
	return ok
}

func excludeUser(userID string) {
	excludedUsers[userID] = true
	saveExclusions()
}

func includeUser(userID string) {
	delete(excludedUsers, userID)
	saveExclusions()
}
