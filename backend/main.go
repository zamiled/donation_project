package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Donation struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Amount    int64     `json:"amount"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}
type Store struct {
	mu        sync.Mutex
	path      string
	donations []Donation
}

func (s *Store) load() error {
	b, e := os.ReadFile(s.path)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	return json.Unmarshal(b, &s.donations)
}
func (s *Store) save() error {
	if e := os.MkdirAll(filepath.Dir(s.path), 0755); e != nil {
		return e
	}
	b, e := json.MarshalIndent(s.donations, "", "  ")
	if e != nil {
		return e
	}
	if e = os.WriteFile(s.path+".tmp", b, 0600); e != nil {
		return e
	}
	return os.Rename(s.path+".tmp", s.path)
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func (s *Store) handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]string{"status": "ok", "mode": "demo"})
	})
	m.HandleFunc("GET /api/profile", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		var total int64
		for _, d := range s.donations {
			total += d.Amount
		}
		write(w, 200, map[string]any{"name": "YOUR STREAM", "bio": "ขอบคุณที่แวะมาดูสตรีมและเป็นกำลังใจให้กัน 💜", "goal": 10000, "goalTitle": "อัปเกรดไมค์สำหรับสตรีม", "total": total, "count": len(s.donations)})
	})
	m.HandleFunc("GET /api/donations", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		out := make([]Donation, 0)
		for i := len(s.donations) - 1; i >= 0 && len(out) < 20; i-- {
			out = append(out, s.donations[i])
		}
		write(w, 200, out)
	})
	m.HandleFunc("POST /api/donations", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var d Donation
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		// Accept only fields supplied by a donor, never client-generated IDs or timestamps.
		var input struct {
			Name    string `json:"name"`
			Amount  int64  `json:"amount"`
			Message string `json:"message"`
		}
		if e := dec.Decode(&input); e != nil {
			write(w, 400, map[string]string{"error": "ข้อมูลไม่ถูกต้อง"})
			return
		}
		if input.Amount < 1 || input.Amount > 1000000 || len([]rune(input.Name)) > 80 || len([]rune(input.Message)) > 300 {
			write(w, 400, map[string]string{"error": "ยอดต้องอยู่ระหว่าง 1–1,000,000 บาท ชื่อไม่เกิน 80 และข้อความไม่เกิน 300 ตัวอักษร"})
			return
		}
		d.Amount = input.Amount
		d.Name = strings.TrimSpace(input.Name)
		if d.Name == "" {
			d.Name = "ผู้บริจาคไม่ประสงค์ออกนาม"
		}
		d.Message = strings.TrimSpace(input.Message)
		d.CreatedAt = time.Now().UTC()
		d.ID = fmt.Sprintf("DEMO-%d", d.CreatedAt.UnixNano())
		s.mu.Lock()
		defer s.mu.Unlock()
		s.donations = append(s.donations, d)
		if e := s.save(); e != nil {
			s.donations = s.donations[:len(s.donations)-1]
			log.Print(e)
			write(w, 500, map[string]string{"error": "บันทึกไม่สำเร็จ กรุณาลองใหม่"})
			return
		}
		write(w, 201, d)
	})
	return m
}
func main() {
	path := os.Getenv("DONATION_DATA_FILE")
	if path == "" {
		path = "data/streamer-donations.json"
	}
	s := &Store{path: path}
	if e := s.load(); e != nil {
		log.Fatal(e)
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Printf("Donation demo listening on %s", addr)
	srv := &http.Server{Addr: addr, Handler: s.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
