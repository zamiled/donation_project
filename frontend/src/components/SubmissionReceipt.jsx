import { useLanguage } from "../i18n/LanguageProvider";
import LanguageSwitcher from "./LanguageSwitcher";
import React from "react";

// ผลสำเร็จของการส่งไฟล์: รายการยังรอตรวจ ไม่ใช่การยืนยันการโอนเงิน
export default function SubmissionReceipt({ receipt, onClose }) {
  const { t, money, displayName } = useLanguage();

  return (
    <div className="backdrop">
      <section
        className="panel receipt"
        role="dialog"
        aria-modal="true"
        aria-labelledby="thanks"
      >
        <div className="modal-language">
          <LanguageSwitcher />
        </div>
        <div className="heart">♥</div>
        <h2 id="thanks">{t("ส่งหลักฐานเรียบร้อย!")} </h2>
        <strong>฿{money(receipt.amount)}</strong>
        <p>
          {t("ขอบคุณ")} {displayName(receipt.name)}{" "}
          {t("กรุณารอเจ้าของสตรีมตรวจสอบหลักฐาน")}{" "}
        </p>
        {receipt.message && <blockquote>{receipt.message}</blockquote>}
        <small>{receipt.id}</small>
        <p className="notice">
          {t("รอตรวจสอบ · ยังไม่นับยอดและยังไม่แสดงแจ้งเตือน")}{" "}
        </p>
        <button autoFocus className="submit" onClick={onClose}>
          {t("กลับไปหน้าโดเนท")}{" "}
        </button>
      </section>
    </div>
  );
}
