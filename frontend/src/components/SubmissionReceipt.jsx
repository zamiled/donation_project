import React from "react";
import { money } from "../lib/format";

// ผลสำเร็จของการส่งไฟล์: รายการยังรอตรวจ ไม่ใช่การยืนยันการโอนเงิน
export default function SubmissionReceipt({ receipt, onClose }) {
  return (
    <div className="backdrop">
      <section
        className="panel receipt"
        role="dialog"
        aria-modal="true"
        aria-labelledby="thanks"
      >
        <div className="heart">♥</div>
        <h2 id="thanks">ส่งหลักฐานเรียบร้อย!</h2>
        <strong>฿{money(receipt.amount)}</strong>
        <p>ขอบคุณ {receipt.name} กรุณารอเจ้าของสตรีมตรวจสอบหลักฐาน</p>
        {receipt.message && <blockquote>{receipt.message}</blockquote>}
        <small>{receipt.id}</small>
        <p className="notice">รอตรวจสอบ · ยังไม่นับยอดและยังไม่แสดงแจ้งเตือน</p>
        <button autoFocus className="submit" onClick={onClose}>
          กลับไปหน้าโดเนท
        </button>
      </section>
    </div>
  );
}
