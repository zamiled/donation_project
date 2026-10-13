package main

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ขั้นที่สอง: รับไฟล์สลิปและเปลี่ยนสถานะเป็นรอตรวจสอบ
// ตรวจความเหมาะสมของไฟล์เท่านั้น ไม่ตรวจว่ามีการโอนเงินจริง
func (s *Store) uploadSlip(w http.ResponseWriter, r *http.Request) {
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
}

// เปิดภาพหลักฐานของรายการสำหรับหน้าตรวจสลิปใน demo
// ยังไม่มีระบบล็อกอินหรือสิทธิ์แอดมิน จึงควรใช้ภาพทดลองเท่านั้น
func (s *Store) getSlip(w http.ResponseWriter, r *http.Request) {
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
}
