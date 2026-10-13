package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// คืนรายการใหม่ก่อน สูงสุด 20 รายการ
// confirmed=1 กรองรายการสำหรับ overlay ไม่ให้รายการรอตรวจถูกแจ้งเตือน
func (s *Store) listDonations(w http.ResponseWriter, r *http.Request) {
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
}

// ขั้นแรก: รับชื่อ ยอด และข้อความ แล้วสร้างรายการรอแนบสลิป
// ยังไม่เพิ่มยอดยืนยันและยังไม่แจ้งเตือน OBS
func (s *Store) createDonation(w http.ResponseWriter, r *http.Request) {
	// จำกัด JSON request ไว้ที่ 4 KiB และปฏิเสธฟิลด์ที่ API ไม่ได้อนุญาต
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var d Donation
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	// รับเฉพาะข้อมูลจากผู้ชม; ID, เวลา, token และสถานะต้องสร้างโดย backend
	var input struct {
		Name      string `json:"name"`
		Anonymous bool   `json:"anonymous"`
		Amount    int64  `json:"amount"`
		Message   string `json:"message"`
	}
	if e := dec.Decode(&input); e != nil {
		write(w, 400, map[string]string{"error": "ข้อมูลไม่ถูกต้อง"})
		return
	}
	// ตรวจชื่อที่ backend ด้วย ป้องกันการข้าม required ในหน้าเว็บหรือเรียก API ตรง
	// เมื่อเลือก anonymous ไม่เก็บชื่อที่ผู้ชมส่งมา เพื่อไม่ให้หลุดไปประวัติหรือ overlay
	if input.Anonymous {
		input.Name = ""
	} else {
		input.Name = strings.TrimSpace(input.Name)
		if input.Name == "" {
			write(w, 400, map[string]string{"error": "กรุณากรอกชื่อที่แสดงบนสตรีม หรือเลือกไม่ประสงค์ออกนาม"})
			return
		}
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
}
