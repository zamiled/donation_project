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

// Donation คือข้อมูลหนึ่งรายการที่ส่งกลับให้หน้าเว็บผ่าน JSON
// สถานะ: awaiting_slip → pending_review → demo_confirmed หรือ rejected
// รายการเก่าที่ไม่มีสถานะยังนับเป็นโดเนททดลองเดิมเพื่อรองรับข้อมูลเวอร์ชันก่อน
type Donation struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Amount    int64     `json:"amount"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
	Status    string    `json:"status"`
	Token     string    `json:"-"`              // สิทธิ์อัปโหลดเฉพาะรายการ: ไม่ส่งออกใน JSON ประวัติทั่วไป
	Slip      string    `json:"slip,omitempty"` // URL เปิดภาพหลักฐาน; ไม่ใช่พาธไฟล์บนเครื่อง
}

// Store เก็บรายการในหน่วยความจำและใช้ไฟล์ JSON แทนฐานข้อมูลสำหรับ demo
// mu ป้องกันคำขอหลายรายการอ่าน/แก้ข้อมูลร่วมกันพร้อมกัน
type Store struct {
	mu        sync.Mutex
	path      string
	donations []Donation
}

// load โหลดข้อมูลเมื่อเริ่ม backend; หากยังไม่มีไฟล์ถือว่าเริ่มจากรายการว่าง
func (s *Store) load() error {
	b, e := os.ReadFile(s.path)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	// ใช้โครงสร้างสำหรับไฟล์โดยเฉพาะ เพื่อกู้ uploadToken ที่ Donation ซ่อนไว้จาก API
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

// save เขียนรายการทั้งหมดลงดิสก์ ผู้เรียกต้องถือ mutex ระหว่างแก้ไขและบันทึก
// แยก uploadToken สำหรับการเก็บไฟล์ โดยไม่เปิดเผยผ่าน JSON ประวัติ
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
	// เขียนไฟล์ชั่วคราวก่อน rename เพื่อลดโอกาสไฟล์หลักเหลือข้อมูลครึ่งเดียว
	// 0600 ให้เฉพาะเจ้าของไฟล์อ่านและเขียนได้
	if e = os.WriteFile(s.path+".tmp", b, 0600); e != nil {
		return e
	}
	return os.Rename(s.path+".tmp", s.path)
}

// write รวมวิธีตอบ API: ตั้งชนิด JSON, HTTP status และเข้ารหัส response
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// handler ลงทะเบียนเส้นทาง REST API; {id} อ่านได้ผ่าน r.PathValue("id")
func (s *Store) handler() http.Handler {
	m := http.NewServeMux()
	// ตรวจว่า backend ตอบคำขอได้และระบุว่าระบบอยู่ในโหมดทดลอง
	m.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]string{"status": "ok", "mode": "demo"})
	})
	// คืนโปรไฟล์ตัวอย่างและยอดรวมเฉพาะรายการยืนยันแล้ว
	// count นับทุกสถานะ รวมรายการที่ยังรอแนบสลิปหรือรอตรวจ
	m.HandleFunc("GET /api/profile", func(w http.ResponseWriter, r *http.Request) {
		// ล็อกข้อมูลร่วมและใช้ defer ปลดล็อกเสมอเมื่อ handler จบ
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
	// คืนรายการใหม่ก่อน สูงสุด 20 รายการ
	// confirmed=1 กรองรายการสำหรับ overlay ไม่ให้รายการรอตรวจถูกแจ้งเตือน
	m.HandleFunc("GET /api/donations", func(w http.ResponseWriter, r *http.Request) {
		// ล็อกข้อมูลร่วมและใช้ defer ปลดล็อกเสมอเมื่อ handler จบ
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
	// ขั้นแรก: รับชื่อ ยอด และข้อความ แล้วสร้างรายการรอแนบสลิป
	// ยังไม่เพิ่มยอดยืนยันและยังไม่แจ้งเตือน OBS
	m.HandleFunc("POST /api/donations", func(w http.ResponseWriter, r *http.Request) {
		// จำกัด JSON request ไว้ที่ 4 KiB และปฏิเสธฟิลด์ที่ API ไม่ได้อนุญาต
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var d Donation
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		// รับเฉพาะข้อมูลจากผู้ชม; ID, เวลา, token และสถานะต้องสร้างโดย backend
		var input struct {
			Name    string `json:"name"`
			Amount  int64  `json:"amount"`
			Message string `json:"message"`
		}
		if e := dec.Decode(&input); e != nil {
			write(w, 400, map[string]string{"error": "ข้อมูลไม่ถูกต้อง"})
			return
		}
		// ตรวจยอดจำนวนเต็มและนับความยาวชื่อ/ข้อความด้วย rune เพื่อรองรับภาษาไทย
		if input.Amount < 1 || input.Amount > 1000000 || len([]rune(input.Name)) > 80 || len([]rune(input.Message)) > 300 {
			write(w, 400, map[string]string{"error": "ยอดต้องอยู่ระหว่าง 1–1,000,000 บาท ชื่อไม่เกิน 80 และข้อความไม่เกิน 300 ตัวอักษร"})
			return
		}
		// ยอดนี้ผูกกับรายการที่สร้าง ไม่มี endpoint สำหรับเปลี่ยนยอดในรายการเดิม
		d.Amount = input.Amount
		d.Name = strings.TrimSpace(input.Name)
		if d.Name == "" {
			d.Name = "ผู้บริจาคไม่ประสงค์ออกนาม"
		}
		d.Message = strings.TrimSpace(input.Message)
		d.CreatedAt = time.Now().UTC()
		// สร้าง token สุ่ม 24 ไบต์สำหรับยืนยันสิทธิ์อัปโหลดสลิปของรายการนี้
		// ใช้ UTC เก็บเวลา ส่วนหน้าเว็บแสดงเวลา Asia/Bangkok
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
		// เพิ่มรายการภายใต้ mutex แล้วบันทึก; ถ้าบันทึกพลาดให้ย้อนรายการในหน่วยความจำ
		s.donations = append(s.donations, d)
		if e := s.save(); e != nil {
			s.donations = s.donations[:len(s.donations)-1]
			log.Print(e)
			write(w, 500, map[string]string{"error": "บันทึกไม่สำเร็จ กรุณาลองใหม่"})
			return
		}
		// คืน token เฉพาะตอนสร้างรายการ และ payload ให้ frontend สร้างรูป QR
		// payload นี้เป็นข้อความทดลอง ไม่ใช่ PromptPay และใช้จ่ายเงินจริงไม่ได้
		write(w, 201, map[string]any{"donation": d, "uploadToken": d.Token, "qrPayload": fmt.Sprintf("PULSE-DEMO|%s|THB|%d.00|NOT-A-PAYMENT-QR", d.ID, d.Amount)})
	})

	// ขั้นที่สอง: รับไฟล์สลิปและเปลี่ยนสถานะเป็นรอตรวจสอบ
	// ตรวจความเหมาะสมของไฟล์เท่านั้น ไม่ตรวจว่ามีการโอนเงินจริง
	m.HandleFunc("POST /api/donations/{id}/slip", func(w http.ResponseWriter, r *http.Request) {
		// จำกัดทั้ง request ไว้ 6 MiB เผื่อ multipart overhead; ตัวไฟล์จำกัด 5 MiB
		r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
		if e := r.ParseMultipartForm(6 << 20); e != nil {
			write(w, 400, map[string]string{"error": "ไฟล์ต้องไม่เกิน 5 MB"})
			return
		}
		// ล้างไฟล์ชั่วคราวที่ multipart parser อาจสร้างหลังคำขอเสร็จ
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		f, _, e := r.FormFile("slip")
		if e != nil {
			write(w, 400, map[string]string{"error": "กรุณาอัปโหลดสลิป"})
			return
		}
		defer f.Close()
		// อ่านเกินเพดานได้เพียง 1 ไบต์ เพื่อแยกไฟล์ใหญ่เกินออกจากไฟล์ที่ขนาดพอดี
		b, e := io.ReadAll(io.LimitReader(f, (5<<20)+1))
		if e != nil || len(b) > 5<<20 {
			write(w, 400, map[string]string{"error": "ไฟล์ต้องไม่เกิน 5 MB"})
			return
		}
		// ตรวจชนิดจากเนื้อไฟล์ ไม่เชื่อชื่อนามสกุลหรือ MIME ที่ผู้ใช้ส่งมา
		mime := http.DetectContentType(b)
		if mime != "image/png" && mime != "image/jpeg" {
			write(w, 400, map[string]string{"error": "รองรับภาพ PNG หรือ JPEG เท่านั้น"})
			return
		}
		// อ่านส่วนหัวภาพเพื่อเช็กขนาดพิกเซลโดยไม่ถอดภาพเต็ม
		// จำกัดด้านละ 10,000 พิกเซลและพื้นที่รวม 25 ล้านพิกเซล
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
			// ต้องมี token ตรงกับรายการ และอัปโหลดได้เฉพาะสถานะรอแนบสลิป
			if d.Token == "" || r.Header.Get("X-Upload-Token") != d.Token {
				write(w, 403, map[string]string{"error": "ไม่มีสิทธิ์อัปโหลดรายการนี้"})
				return
			}
			if d.Status != "awaiting_slip" {
				write(w, 409, map[string]string{"error": "รายการนี้ส่งสลิปแล้ว"})
				return
			}
			// ตั้งชื่อไฟล์จาก ID ที่ server สร้าง ไม่ใช้ชื่อไฟล์จากผู้ชม
			// 0700 จำกัดสิทธิ์โฟลเดอร์ และ 0600 จำกัดสิทธิ์ไฟล์
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
			// เก็บข้อมูลเดิมไว้ย้อนกลับ หากบันทึก JSON ไม่สำเร็จให้ลบภาพที่เพิ่งเขียนด้วย
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
	// เปิดภาพหลักฐานของรายการสำหรับหน้าตรวจสลิปใน demo
	// ยังไม่มีระบบล็อกอินหรือสิทธิ์แอดมิน จึงควรใช้ภาพทดลองเท่านั้น
	m.HandleFunc("GET /api/donations/{id}/slip", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, d := range s.donations {
			if d.ID == r.PathValue("id") && d.Slip != "" {
				// ไม่ให้ browser เดาชนิดไฟล์เอง และไม่เก็บภาพหลักฐานใน cache
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.Header().Set("Cache-Control", "no-store")
				http.ServeFile(w, r, filepath.Join(filepath.Dir(s.path), "slips", d.ID))
				return
			}
		}
		http.NotFound(w, r)
	})
	// ขั้นที่สาม: รับผลตรวจด้วยตัวเอง approve/reject จากแดชบอร์ด
	// approve หมายถึงยืนยันใน demo ไม่ใช่ผลตรวจจากธนาคารหรือบริการตรวจสลิป
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
			// ตรวจลำดับสถานะเพื่อห้ามยืนยันก่อนส่งสลิป และห้ามตรวจรายการเดิมซ้ำ
			if d.Status != "pending_review" || d.Slip == "" {
				write(w, 409, map[string]string{"error": "ต้องส่งสลิปก่อนตรวจสอบ"})
				return
			}
			// หากบันทึกผลตรวจล้มเหลว ให้คืนสถานะเดิมเพื่อไม่ให้ยอดในหน่วยความจำผิด
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

// จุดเริ่มโปรแกรม: อ่าน configuration, กู้รายการเดิม และเริ่ม HTTP server
func main() {
	// เปลี่ยนตำแหน่งข้อมูลได้ผ่าน DONATION_DATA_FILE; ค่าเริ่มต้นแยกจาก demo รุ่นเก่า
	path := os.Getenv("DONATION_DATA_FILE")
	if path == "" {
		path = "data/streamer-donations.json"
	}
	s := &Store{path: path}
	if e := s.load(); e != nil {
		log.Fatal(e)
	}
	// ADDR เปลี่ยนพอร์ต/ที่อยู่ได้; ค่าเริ่มต้นรับเฉพาะการเชื่อมต่อภายในเครื่อง
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Printf("Donation demo listening on %s", addr)
	// กำหนด timeout เพื่อลดการค้างของคำขอ; ListenAndServe ทำงานจน server หยุด
	srv := &http.Server{Addr: addr, Handler: s.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
