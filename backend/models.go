package main

import (
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
