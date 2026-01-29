package web

import (
    "encoding/json"
    "net/http"
)

func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}, status int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if data != nil {
        json.NewEncoder(w).Encode(data)
    }
}

func (s *Server) errorResponse(w http.ResponseWriter, err error, status int) {
    http.Error(w, err.Error(), status)
}

func (s *Server) badRequest(w http.ResponseWriter, err error) {
    s.errorResponse(w, err, http.StatusBadRequest)
}

func (s *Server) serverError(w http.ResponseWriter, err error) {
    s.errorResponse(w, err, http.StatusInternalServerError)
}
