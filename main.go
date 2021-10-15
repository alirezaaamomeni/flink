package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

var TTL = 60 * time.Second

type location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
	createTime time.Time `json:"-"`
}

type handler struct {
	l sync.Mutex
	history map[string][]location
}

func handleRequests(r *mux.Router, h *handler) {
	sub := r.PathPrefix("/location").Subrouter()
	sub.HandleFunc("/{id}/now", h.addHistory).Methods("POST")
	sub.HandleFunc("/{id}", h.getHistory).Methods("GET")
	sub.HandleFunc("/{id}", h.deleteHistory).Methods("DELETE")
	adr := os.Getenv("HISTORY_SERVER_LISTEN_ADDR")
	if adr == "" {
		adr = "8080"
	}
	a := fmt.Sprintf(":%s", adr)
	log.Fatal(http.ListenAndServe(a, r))
}

func (h *handler) addHistory(w http.ResponseWriter, r *http.Request){
	id := mux.Vars(r)["id"]

	loc := location{}
	if err := json.NewDecoder(r.Body).Decode(&loc); err != nil {
		http.Error(w, "Error in deserialize the request", http.StatusBadRequest)
	}
	fmt.Println(loc)
	loc.createTime = time.Now()

	if h.history[id] == nil {
		h.history[id] = make([]location,0)
	}
	h.l.Lock()
	defer h.l.Unlock()
	h.history[id] = append(h.history[id], loc)
}

func (h *handler) deleteHistory(w http.ResponseWriter, r *http.Request){
	id := mux.Vars(r)["id"]
	_, ok := h.history[id]
	if !ok {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}
	h.l.Lock()
	defer h.l.Unlock()
	h.history[id] = nil
}

func (h *handler) getHistory(w http.ResponseWriter, r *http.Request){
	id := mux.Vars(r)["id"]
	var l int
	if v, ok := r.URL.Query()["max"]; ok {
		l,_ = strconv.Atoi(v[0])
	}

	history, ok := h.history[id]
	if !ok {
		http.Error(w, "Error in found history for order id", http.StatusNotFound)
	}
	res := make([]location, 0)
	for _, e := range history {
		if time.Now().Sub(e.createTime) < TTL {
			res = append(res, e)
		}
	}
	if l != 0 {
		res = res[len(history) - l:]
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, "Error in serializing...", http.StatusInternalServerError)
	}
}


func main() {
	ttl := os.Getenv("LOCATION_HISTORY_TTL_SECONDS")
	if ttl != "" {
		TTL, _ = time.ParseDuration(os.Getenv("LOCATION_HISTORY_TTL_SECONDS"))
	}
	history := make(map[string][]location)
	h := handler{history: history}
	r := mux.NewRouter()
	handleRequests(r, &h)
}
