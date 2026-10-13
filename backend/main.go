package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

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
