import { useLanguage } from "../i18n/LanguageProvider";
import React from "react";
import DonationHistory from "../components/DonationHistory";

// หน้าสตรีมเมอร์: สถิติ URL สำหรับ OBS และรายการสำหรับตรวจหลักฐาน
export default function DashboardPage({
  profile,
  items,
  copied,
  copy,
  error,
  refresh,
}) {
  const { t, money } = useLanguage();

  return (
    <section className="dashboard">
      <div className="eyebrow">{t("CREATOR SPACE · DEMO")} </div>
      <h1>{t("แดชบอร์ดสตรีมเมอร์")} </h1>
      <p className="muted">{t("ยอดและข้อความจากผู้ชม อัปเดตทุก 3 วินาที")} </p>
      {/* ยอดนับเฉพาะยืนยันแล้ว แต่จำนวนรายการรวมสถานะที่ส่งหลักฐานแล้วตาม profile.count */}
      <div className="metrics">
        <section className="panel">
          <span>{t("ยอดยืนยันแล้ว (ทดลอง)")} </span>
          <strong>฿{money(profile.total)}</strong>
        </section>
        <section className="panel">
          <span>{t("รายการทั้งหมด / รอตรวจรวม")} </span>
          <strong>{profile.count}</strong>
        </section>
        <section className="panel">
          <span>{t("เป้าหมายสตรีม")} </span>
          <strong>{Math.round((profile.total / profile.goal) * 100)}%</strong>
        </section>
      </div>
      <section className="panel obs-panel">
        <div>
          <div className="eyebrow">{t("OBS BROWSER SOURCE")} </div>
          <h2>{t("แจ้งเตือนโดเนทบนสตรีม")} </h2>
          <p>
            {t("เพิ่ม Browser Source ขนาด 800 × 400 ใน OBS แล้วใช้ URL นี้")}{" "}
            <br />
            {t(
              "เปิด overlay ก่อนส่งโดเนท รายการใหม่จะแสดงตามลำดับครั้งละ 7 วินาที",
            )}{" "}
          </p>
          <code>{location.origin}/overlay</code>
        </div>
        <button className="secondary" onClick={copy}>
          {copied ? t("คัดลอกแล้ว ✓") : t("คัดลอก URL")}
        </button>
      </section>
      {error && <p className="error">{t(error)}</p>}
      <section className="panel">
        <h2>
          {t("รายการล่าสุด")} <small>{t("20 รายการ")} </small>
        </h2>
        {/* ส่ง refresh เป็น callback เพื่อโหลดรายการใหม่หลังยืนยัน/ปฏิเสธสลิป */}
        <DonationHistory items={items} onReview={refresh} />
      </section>
      <p className="form-note">
        {t(
          "การยืนยันนี้เป็นการตรวจด้วยตัวเองใน demo ไม่ได้ตรวจยอด ผู้รับ หรือสลิปซ้ำอัตโนมัติ · ยังไม่มีล็อกอิน",
        )}{" "}
      </p>
    </section>
  );
}
