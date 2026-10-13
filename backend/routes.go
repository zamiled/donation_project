package main

import (
	"net/http"
)

// handler รวมเส้นทาง API โดยแยกการทำงานจริงไว้ในไฟล์ *_handler.go
func (s *Store) handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/health", s.health)
	m.HandleFunc("GET /api/profile", s.profile)
	m.HandleFunc("GET /api/donations", s.listDonations)
	m.HandleFunc("POST /api/donations", s.createDonation)
	m.HandleFunc("POST /api/donations/{id}/slip", s.uploadSlip)
	m.HandleFunc("GET /api/donations/{id}/slip", s.getSlip)
	m.HandleFunc("POST /api/donations/{id}/review", s.reviewSlip)
	return m
}
