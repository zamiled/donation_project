import React, { useState } from "react";
import { api } from "../lib/api";
import { money } from "../lib/format";

// แปลงสถานะจาก backend ให้เป็นข้อความภาษาไทย โดยไม่เปลี่ยนค่าที่เก็บจริง
const statuses = {
  awaiting_slip: "รอแนบสลิป",
  pending_review: "รอตรวจสอบ",
  demo_confirmed: "ยืนยันแล้ว (ทดลอง)",
  rejected: "ไม่ผ่าน",
};
// ใช้ component เดียวกันทั้งประวัติผู้ชมและแดชบอร์ด
// onReview ส่งมาเฉพาะแดชบอร์ด เพื่อเปิดลิงก์สลิปและปุ่มตรวจหลักฐาน
export default function DonationHistory({ items, onReview }) {
  const [busy, setBusy] = useState(""),
    [error, setError] = useState("");
  // ส่งผล approve/reject แล้วเรียก callback โหลดข้อมูลใหม่
  // busy เก็บ ID ที่กำลังส่งเพื่อปิดปุ่มของรายการนั้น ส่วน error ใช้แสดงผลล้มเหลว
  async function review(id, decision) {
    setBusy(id);
    setError("");
    try {
      await api("donations/" + id + "/review", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ decision }),
      });
      await onReview();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy("");
    }
  }
  return (
    <>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {/* มีรายการให้แสดงรายการตามลำดับที่ API ส่งมา; ถ้าไม่มีให้แสดงสถานะว่าง */}
      {items.length ? (
        <div className="history">
          {items.map((d) => (
            <article key={d.id}>
              <span className="donor-icon">♥</span>
              <div>
                <strong>{d.name}</strong>
                {/* แปลงเวลา UTC จาก backend เป็นเวลาไทย Asia/Bangkok */}
                <small>
                  {new Date(d.createdAt).toLocaleString("th-TH", {
                    timeZone: "Asia/Bangkok",
                    dateStyle: "short",
                    timeStyle: "short",
                  })}
                </small>
                {d.message && <p>{d.message}</p>}
                <small className={"status " + d.status}>
                  {statuses[d.status] || "โดเนททดลองเดิม"}
                </small>
                {onReview && d.slip && (
                  <a
                    className="slip-link"
                    href={d.slip}
                    target="_blank"
                    rel="noreferrer"
                  >
                    ดูสลิป ↗
                  </a>
                )}
                {/* เปิดปุ่มตรวจเฉพาะรายการรอตรวจ; backend ตรวจสถานะซ้ำอีกชั้น */}
                {onReview && d.status === "pending_review" && (
                  <div className="review-buttons">
                    <button
                      disabled={busy === d.id}
                      onClick={() => review(d.id, "approve")}
                    >
                      ยืนยันหลักฐาน (ทดลอง)
                    </button>
                    <button
                      disabled={busy === d.id}
                      onClick={() => review(d.id, "reject")}
                    >
                      ไม่ผ่าน
                    </button>
                  </div>
                )}
              </div>
              <b>฿{money(d.amount)}</b>
            </article>
          ))}
        </div>
      ) : (
        <div className="empty">
          <span>♡</span>
          <p>ยังไม่มีโดเนท</p>
          <small>ส่งกำลังใจแรกให้สตรีมนี้กัน</small>
        </div>
      )}
    </>
  );
}
