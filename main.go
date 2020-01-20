package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// Vote represents a vote in the blockchain
type Vote struct {
	Index     int       `json:"index"`
	Timestamp time.Time `json:"timestamp"`
	Value     string    `json:"value"`
	Hash      string    `json:"hash"`
	PrevHash  string    `json:"prev_hash"`
}

// Votes represents a blockchain
var Votes []Vote

// Request represents an input vote
type Request struct {
	Value string `json:"value"`
}

var mutex = &sync.Mutex{}

func main() {
	log.Printf("starting service...")
	apiPort := flag.String("port", "9000", "specifies the API port")
	flag.Parse()

	go func() {
		genesisVote := Vote{
			Index:     0,
			Timestamp: time.Now(),
			Value:     "",
			PrevHash:  "",
		}

		genesisVote.Hash = generateHash(genesisVote)

		mutex.Lock()
		Votes = append(Votes, genesisVote)
		mutex.Unlock()
	}()

	log.Printf("service is ready. listening on port: %s", *apiPort)
	err := startServer(*apiPort)
	if err != http.ErrServerClosed {
		log.Fatalf("server failed to start: %v", err)
	}
}

func startServer(port string) error {
	server := &http.Server{
		Addr:    ":" + port,
		Handler: setRouter(),
	}

	return server.ListenAndServe()
}

func setRouter() *mux.Router {
	router := mux.NewRouter()

	router.HandleFunc("/vote", getVotes).Methods("GET")
	router.HandleFunc("/vote", postVote).Methods("POST")

	return router
}

func getVotes(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, Votes)
}

func postVote(w http.ResponseWriter, r *http.Request) {
	var req Request

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req)
	if err != nil {
		respondWithJSON(w, http.StatusBadRequest, r.Body)
		return
	}
	defer r.Body.Close()

	mutex.Lock()
	prevVote := Votes[len(Votes)-1]
	newVote := createVote(prevVote, req.Value)

	valid := validateVote(newVote, prevVote)
	if !valid {
		respondWithJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{"message": "invalid vote"})
		return
	}

	Votes = append(Votes, newVote)

	mutex.Unlock()

	respondWithJSON(w, http.StatusOK, Votes)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to marshal: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err = w.Write(response)
	if err != nil {
		return
	}
}

func generateHash(vote Vote) string {
	v := fmt.Sprintf("%v%v%v%v", vote.Index, vote.Timestamp, vote.Value, vote.PrevHash)

	hasher := sha256.New()
	hasher.Write([]byte(v))

	return hex.EncodeToString(hasher.Sum(nil))
}

func createVote(oldVote Vote, s string) Vote {
	vote := Vote{
		Index:     oldVote.Index + 1,
		Timestamp: time.Now(),
		Value:     s,
		PrevHash:  oldVote.Hash,
	}

	vote.Hash = generateHash(vote)

	return vote
}

func validateVote(newVote, oldVote Vote) bool {
	if oldVote.Index+1 != newVote.Index {
		return false
	}

	if oldVote.Hash != newVote.PrevHash {
		return false
	}

	if generateHash(newVote) != newVote.Hash {
		return false
	}

	return true
}
