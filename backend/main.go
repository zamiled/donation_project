package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
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
	Status    string    `json:"status"`
	Token     string    `json:"-"`
	Slip      string    `json:"slip,omitempty"`
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
	var saved []struct {
		Donation
		UploadToken string `json:"uploadToken"`
	}
	if e := json.Unmarshal(b, &saved); e != nil {
		return e
	}
	for _, d := range saved {
		d.Donation.Token = d.UploadToken
		s.donations = append(s.donations, d.Donation)
	}
	return nil
}
func (s *Store) save() error {
	if e := os.MkdirAll(filepath.Dir(s.path), 0755); e != nil {
		return e
	}
	saved := make([]struct {
		Donation
		UploadToken string `json:"uploadToken"`
	}, 0, len(s.donations))
	for _, d := range s.donations {
		saved = append(saved, struct {
			Donation
			UploadToken string `json:"uploadToken"`
		}{d, d.Token})
	}
	b, e := json.MarshalIndent(saved, "", "  ")
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
			if d.Status == "demo_confirmed" || d.Status == "" {
				total += d.Amount
			}
		}
		write(w, 200, map[string]any{"name": "YOUR STREAM", "bio": "ขอบคุณที่แวะมาดูสตรีมและเป็นกำลังใจให้กัน 💜", "goal": 10000, "goalTitle": "อัปเกรดไมค์สำหรับสตรีม", "total": total, "count": len(s.donations)})
	})
	m.HandleFunc("GET /api/donations", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		out := make([]Donation, 0)
		for i := len(s.donations) - 1; i >= 0 && len(out) < 20; i-- {
			if r.URL.Query().Get("confirmed") == "1" && s.donations[i].Status != "demo_confirmed" && s.donations[i].Status != "" {
				continue
			}
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
		raw := make([]byte, 24)
		if _, e := rand.Read(raw); e != nil {
			write(w, 500, map[string]string{"error": "สร้างรายการไม่สำเร็จ"})
			return
		}
		d.Token = hex.EncodeToString(raw)
		d.ID = fmt.Sprintf("DEMO-%d", d.CreatedAt.UnixNano())
		d.Status = "awaiting_slip"
		s.mu.Lock()
		defer s.mu.Unlock()
		s.donations = append(s.donations, d)
		if e := s.save(); e != nil {
			s.donations = s.donations[:len(s.donations)-1]
			log.Print(e)
			write(w, 500, map[string]string{"error": "บันทึกไม่สำเร็จ กรุณาลองใหม่"})
			return
		}
		write(w, 201, map[string]any{"donation": d, "uploadToken": d.Token, "qrPayload": fmt.Sprintf("PULSE-DEMO|%s|THB|%d.00|NOT-A-PAYMENT-QR", d.ID, d.Amount)})
	})

	m.HandleFunc("POST /api/donations/{id}/slip", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
		if e := r.ParseMultipartForm(6 << 20); e != nil {
			write(w, 400, map[string]string{"error": "ไฟล์ต้องไม่เกิน 5 MB"})
			return
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		f, _, e := r.FormFile("slip")
		if e != nil {
			write(w, 400, map[string]string{"error": "กรุณาอัปโหลดสลิป"})
			return
		}
		defer f.Close()
		b, e := io.ReadAll(io.LimitReader(f, (5<<20)+1))
		if e != nil || len(b) > 5<<20 {
			write(w, 400, map[string]string{"error": "ไฟล์ต้องไม่เกิน 5 MB"})
			return
		}
		mime := http.DetectContentType(b)
		if mime != "image/png" && mime != "image/jpeg" {
			write(w, 400, map[string]string{"error": "รองรับภาพ PNG หรือ JPEG เท่านั้น"})
			return
		}
		cfg, _, e := image.DecodeConfig(strings.NewReader(string(b)))
		if e != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 10000 || cfg.Height > 10000 || int64(cfg.Width)*int64(cfg.Height) > 25000000 {
			write(w, 400, map[string]string{"error": "ไฟล์ภาพไม่ถูกต้องหรือมีขนาดภาพใหญ่เกินไป"})
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		for i := range s.donations {
			d := &s.donations[i]
			if d.ID != r.PathValue("id") {
				continue
			}
			if d.Token == "" || r.Header.Get("X-Upload-Token") != d.Token {
				write(w, 403, map[string]string{"error": "ไม่มีสิทธิ์อัปโหลดรายการนี้"})
				return
			}
			if d.Status != "awaiting_slip" {
				write(w, 409, map[string]string{"error": "รายการนี้ส่งสลิปแล้ว"})
				return
			}
			dir := filepath.Join(filepath.Dir(s.path), "slips")
			if e := os.MkdirAll(dir, 0700); e != nil {
				write(w, 500, map[string]string{"error": "บันทึกไฟล์ไม่สำเร็จ"})
				return
			}
			path := filepath.Join(dir, d.ID)
			if e := os.WriteFile(path, b, 0600); e != nil {
				write(w, 500, map[string]string{"error": "บันทึกไฟล์ไม่สำเร็จ"})
				return
			}
			old := *d
			d.Slip = "/api/donations/" + d.ID + "/slip"
			d.Status = "pending_review"
			if e := s.save(); e != nil {
				*d = old
				os.Remove(path)
				write(w, 500, map[string]string{"error": "บันทึกรายการไม่สำเร็จ"})
				return
			}
			write(w, 200, d)
			return
		}
		write(w, 404, map[string]string{"error": "ไม่พบรายการ"})
	})
	m.HandleFunc("GET /api/donations/{id}/slip", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, d := range s.donations {
			if d.ID == r.PathValue("id") && d.Slip != "" {
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.Header().Set("Cache-Control", "no-store")
				http.ServeFile(w, r, filepath.Join(filepath.Dir(s.path), "slips", d.ID))
				return
			}
		}
		http.NotFound(w, r)
	})
	m.HandleFunc("POST /api/donations/{id}/review", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Decision string `json:"decision"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		if e := json.NewDecoder(r.Body).Decode(&input); e != nil || (input.Decision != "approve" && input.Decision != "reject") {
			write(w, 400, map[string]string{"error": "ระบุ approve หรือ reject"})
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		for i := range s.donations {
			d := &s.donations[i]
			if d.ID != r.PathValue("id") {
				continue
			}
			if d.Status != "pending_review" || d.Slip == "" {
				write(w, 409, map[string]string{"error": "ต้องส่งสลิปก่อนตรวจสอบ"})
				return
			}
			old := d.Status
			if input.Decision == "approve" {
				d.Status = "demo_confirmed"
			} else {
				d.Status = "rejected"
			}
			if e := s.save(); e != nil {
				d.Status = old
				write(w, 500, map[string]string{"error": "บันทึกผลไม่สำเร็จ"})
				return
			}
			write(w, 200, d)
			return
		}
		write(w, 404, map[string]string{"error": "ไม่พบรายการ"})
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
