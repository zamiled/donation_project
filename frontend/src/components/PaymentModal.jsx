import { useLanguage } from "../i18n/LanguageProvider";
import React, { useEffect, useState } from "react";
import QRCode from "qrcode";
import { api } from "../lib/api";

// ใช้ภาษาร่วมจากหน้าโดเนท ไม่มีตัวเลือกภาษาแยกในหน้าต่าง QR
// หน้าต่างขั้น QR และสลิป: order มาจาก backend จึงยึดยอดและ ID ของรายการนั้น
// onClose กลับไปแก้ยอด; onComplete แจ้ง App ว่าอัปโหลดสำเร็จและรอตรวจ
export default function PaymentModal({ order, onClose, onComplete }) {
  const { t, money } = useLanguage();

  // qr คือรูป QR แบบ data URL; file คือไฟล์ที่เลือก; preview คือ URL สำหรับดูภาพก่อนส่ง
  const [qr, setQr] = useState(""),
    [file, setFile] = useState(null),
    [preview, setPreview] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    let stopped = false;
    // สร้างรูปจากข้อความ payload ทดลอง ไม่ใช่ QR PromptPay
    // errorCorrectionLevel M ช่วยให้ QR อ่านได้แม้บางส่วนมีความเสียหายเล็กน้อย
    QRCode.toDataURL(order.qrPayload, {
      width: 260,
      margin: 2,
      errorCorrectionLevel: "M",
    })
      .then((url) => {
        if (!stopped) setQr(url);
      })
      .catch(() => setError("สร้าง QR ไม่สำเร็จ"));
    return () => {
      stopped = true;
    };
  }, [order.qrPayload]);
  useEffect(() => {
    if (!file) {
      setPreview("");
      return;
    }
    // สร้าง URL ชั่วคราวของไฟล์ใน browser เพื่อแสดง preview โดยยังไม่อัปโหลด
    // cleanup คืนทรัพยากรเมื่อเปลี่ยนไฟล์หรือปิดหน้าต่าง
    const url = URL.createObjectURL(file);
    setPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);
  // ตรวจ MIME และขนาดก่อนแสดง preview เพื่อแจ้งผู้ใช้ทันที
  // backend ยังต้องตรวจจากเนื้อไฟล์จริง เพราะการตรวจใน browser ถูกข้ามได้
  function select(e) {
    const f = e.target.files?.[0];
    setError("");
    setFile(null);
    if (!f) return;
    if (
      !["image/png", "image/jpeg"].includes(f.type) ||
      f.size > 5 * 1024 * 1024
    ) {
      setError("เลือกภาพ PNG หรือ JPEG ขนาดไม่เกิน 5 MB");
      e.target.value = "";
      return;
    }
    setFile(f);
  }
  // ส่งภาพแบบ multipart/form-data; ไม่ตั้ง Content-Type เอง ให้ browser ใส่ boundary
  // X-Upload-Token พิสูจน์สิทธิ์อัปโหลดเฉพาะรายการ ไม่ใช่สิทธิ์ตรวจสลิปของแอดมิน
  async function upload(e) {
    e.preventDefault();
    if (!file) return;
    setBusy(true);
    setError("");
    try {
      const data = new FormData();
      data.append("slip", file);
      const result = await api("donations/" + order.donation.id + "/slip", {
        method: "POST",
        headers: { "X-Upload-Token": order.uploadToken },
        body: data,
      });
      onComplete(result);
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="backdrop">
      <section
        className="panel payment"
        role="dialog"
        aria-modal="true"
        aria-labelledby="payment-title"
      >
        <div className="eyebrow">{t("STEP 2 · QR & SLIP")} </div>
        <h2 id="payment-title">
          {t("QR สำหรับยอด ฿")}
          {money(order.donation.amount)}
        </h2>
        <p className="notice">{t("QR ทดลองเท่านั้น ใช้จ่ายเงินจริงไม่ได้")} </p>
        {qr ? (
          <img
            className="qr"
            src={qr}
            alt={t("QR ทดลองยอด ") + order.donation.amount + t(" บาท")}
          />
        ) : (
          <p>{t("กำลังสร้าง QR…")} </p>
        )}
        <small className="order-id">{order.donation.id}</small>
        <p className="muted">
          {t(
            "ยอดนี้ล็อกตามรายการ หากต้องการเปลี่ยนยอดให้กลับไปสร้างรายการใหม่",
          )}{" "}
        </p>
        {/* ส่งได้เมื่อเลือกไฟล์แล้ว; อัปโหลดสำเร็จหมายถึงรอตรวจ ไม่ได้ยืนยันว่าโอนเงินจริง */}
        <form onSubmit={upload}>
          <label>
            {t("อัปโหลดหลักฐานทดลอง")}{" "}
            <input
              type="file"
              accept="image/png,image/jpeg"
              onChange={select}
              disabled={busy}
              required
            />
          </label>
          <small className="muted">
            {t(
              "PNG / JPEG ไม่เกิน 5 MB · ใช้ภาพตัวอย่าง หลีกเลี่ยงข้อมูลบัญชีจริง",
            )}{" "}
          </small>
          {preview && (
            <img
              className="slip-preview"
              src={preview}
              alt={t("ตัวอย่างสลิปที่เลือก")}
            />
          )}
          {error && (
            <p className="error" role="alert">
              {t(error)}
            </p>
          )}
          <button className="submit" disabled={!file || busy}>
            {busy ? t("กำลังอัปโหลด…") : t("ส่งสลิปเพื่อรอตรวจสอบ")}
          </button>
          <button
            type="button"
            className="secondary back-button"
            disabled={busy}
            onClick={onClose}
          >
            {t("กลับไปแก้ไขยอด")}{" "}
          </button>
        </form>
      </section>
    </div>
  );
}
