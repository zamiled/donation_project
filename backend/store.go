package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

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
